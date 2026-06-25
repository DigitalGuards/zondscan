# Faucet funding-key security design

Status: parked design notes, 2026-06-25. Current implementation is Rung 0 (below). No code changed from this discussion; this is a record of the design space and the agreed direction.

## Context

The testnet faucet (`/faucet`) signs and broadcasts native transfers server-side from a funded QRL account. Code: `ExplorerFrontend/app/lib/faucet.ts` and `ExplorerFrontend/app/faucet/claim/route.ts`. The funding key is `FAUCET_SEED` in `ExplorerFrontend/.env`.

Today `FAUCET_SEED` is the nft-deploy deployer wallet (~25.5k testnet QRL), which is also used by deploy tooling. So one wallet is acting as both a treasury and an internet-facing hot wallet.

## Threat model (stated precisely)

- The seed is server-only. It is read via a non-`NEXT_PUBLIC` `process.env` var, behind `import 'server-only'` + `runtime = 'nodejs'`, so it lives only in the Next.js server process and is never shipped to browsers (verified: 0 references in the built client chunks). A client-side bug (XSS and friends) cannot reach it.
- The real exposure is server-side. An RCE, arbitrary-file-read, SSRF, or accidental `process.env` dump anywhere in the Next server, or anyone with box access, reads the plaintext seed and drains the entire balance on-chain in a single transaction.
- Abuse of the faucet endpoint itself is bounded (10 per claim, captcha, per-IP and per-address cooldown, daily cap). The danger is seed exposure, not endpoint abuse.
- Aggravating factors: the seed sits in the highest-surface process available (Next SSR, renders user-influenced content, many dependencies, internet-facing), and it is shared with the deploy wallet.

## Core invariant

An auto-signing faucet needs some online key with spend authority. The design goal is not "no hot key" but "isolate it and bound the blast radius." Three roughly-independent levers:

1. Blast radius (what a full compromise costs): float vs treasury, daily cap.
2. Attack surface (how many ways to reach the key): which process holds it, as which OS user, `.env` permissions.
3. Key vs capability (does the component hold the raw seed, or only the ability to request a capped signature): signer service, contract, KMS.

## Hardening ladder (cheap to strong)

- Rung 0 (current): raw seed in the Next server `.env`. Surface and secret co-located; the whole balance is at risk on any server-side compromise.
- Rung 1 (cheap, most of the win): dedicated faucet-only wallet with a small float; treasury stays cold; `.env` `chmod 600` owned by the service user; `FAUCET_DAILY_CAP_QUANTA` set near the float. Caps loss at the float and un-shares the deploy wallet. Rotating `FAUCET_SEED` is server-only runtime env, so it only needs `pm2 restart frontend`, no rebuild.
- Rung 2 (privilege separation): move signing into a tiny separate process, own OS user, localhost-only, single `drip(addr)` endpoint, enforces cooldown/cap itself. A web-app RCE no longer hands over the key; an attacker can at most call the rate-limited signer. Limit is still code-enforced, so an attacker who fully owns the signer process can bypass it.
- Rung 3 (KMS/HSM): key never leaves a signing oracle. Constraint: no managed KMS signs ML-DSA-87, so for QRL this collapses back into Rung 2 (run your own signer). Not available off the shelf today.

## Target design: on-chain faucet contract

Why it beats Rung 2: on-chain caps are trustless. In Rung 2 the cap is enforced by your code, which an attacker holding the key bypasses. With a contract, the chain enforces the cap, so even a fully compromised operator key cannot exceed it. The limit moves from "trusted code an attacker can skip" to "a rule the network enforces regardless."

Shape:

- Funds held in the contract.
- `owner` = cold wallet: `withdraw` / sweep, `setOperator`, `setDripAmount`, `setRateLimit`, `pause`. The only role that can move large amounts. This is the "only my cold wallet can take more than X at once" property.
- `operator` = hot key on the faucet server: can only call `drip(recipient)`, which sends a fixed `dripAmount`, gated on-chain by a per-recipient cooldown and a global per-window cap.
- The server still relays and pays gas (so a zero-balance user can receive) and still runs captcha + per-IP cooldown off-chain.
- Operator-key compromise is bounded to the per-window cap; rotate it via `setOperator` from cold. The treasury is never exposed.

Caveats:

- Trades key-custody risk for contract-correctness risk. A logic bug can lose the entire contract balance in one transaction. So the contract is not the treasury: fund it with a working balance and top up from cold.
- The hot key is not eliminated, only demoted from a treasury key to a capped-drip key.
- The chain cannot do Sybil resistance (no captcha, no IP). Keep the off-chain layer for fairness; the contract backstops custody.
- The operator EOA needs a small gas float.
- Do not make `drip` permissionless: zero-balance users cannot pay gas to call it. Server-relayed is the right variant.

## "Only drip via the explorer" and contract-compromise containment

Making `drip()` `onlyOperator` (so only the explorer's key can trigger it) is good hygiene, but it does not defend against the contract being compromised:

- "Contract compromise" is one of two things: a logic bug (missing access check, reentrancy, a function that should be `onlyOwner` and is not), or a leaked privileged key. `onlyOperator` only constrains the `drip` path. A logic bug elsewhere is exploited by calling the flawed function directly from any address; the attacker never touches `drip()`, so gating it is irrelevant to that case.
- The contract cannot authenticate "the explorer." On-chain it only knows who signed the transaction (the operator address). "Only via the explorer" really means "only from the operator key." If that key leaks, anyone calls `drip` from anywhere and the contract cannot tell the difference.

Real containment for a fund-holding contract, all on-chain:

1. Keep the bulk cold; fund the contract with a working balance only. This is the single biggest lever, because it is the only thing that helps against the bug case that bypasses everything else.
2. On-chain global per-window cap: bounds operator-key compromise.
3. `onlyOwner` (cold) for `withdraw`, params, and `setOperator`.
4. Pause / circuit breaker. Caveat: pause only stops paths that check `whenNotPaused`; a bug that skips the modifier is not stopped.
5. Immutable, not upgradeable. A proxy upgrade key is a god key and a classic exploit target; for a small-balance faucet, prefer immutable and redeploy + re-fund if a bug is found.
6. Audit and keep it tiny: OpenZeppelin `Ownable`/`Pausable`, checks-effects-interactions.

Honest tension: the contract's value is trustless caps that survive a server breach. You cannot also make the contract defer to the explorer without going back to trusting server code. The realistic posture is defense-in-depth with two independent caps: the contract enforces a hard on-chain cap (survives a server breach), and the explorer enforces captcha/IP/cooldown before relaying (handles Sybil). Each covers the other's blind spot, except a fund-draining contract bug, which only "small balance + pause + audit + redeploy" addresses.

## Proportionality

Testnet QRL has no monetary value and is trivially refilled, so Rung 1 is the proportionate stopping point today. The contract design is over-engineered for a testnet-only faucet. It becomes the correct design once a faucet holds real value (mainnet). Build and audit it on testnet so it is ready, reusing the QRC20-Factory / Hyperion toolchain; the same artifact then serves mainnet.

## Recommended path

1. When desired: Rung 1. Dedicated low-float faucet wallet, `.env` `chmod 600`, `FAUCET_DAILY_CAP_QUANTA` near the float. Rotating the seed is `pm2 restart frontend` only (server-only runtime env, no rebuild).
2. Before mainnet: build the on-chain faucet contract (owner = cold, operator = hot, per-recipient cooldown, global window cap, pausable, immutable), keep most funds cold and top up, run it through a smart-contract review, and integrate the explorer as the relayer.

## Status

Parked 2026-06-25. Current state is Rung 0. No code changes have been made from this design discussion.
