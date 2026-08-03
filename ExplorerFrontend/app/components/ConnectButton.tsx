"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { QRCodeCanvas } from "qrcode.react";
import {
  getQrlConnect,
  ConnectionStatus,
  type QRLConnectProvider,
} from "../lib/qrlConnect";
import {
  getRestorableWalletSession,
  parseAuthorizedQrlAccount,
  requestAuthorizedQrlAccount,
} from "../lib/qrlAccounts";

interface ConnectButtonProps {
  /** Called whenever the connected account changes (or null on disconnect). */
  onAccount?: (account: string | null) => void;
  /** Optional: expose the underlying provider so callers can issue requests. */
  onProvider?: (provider: QRLConnectProvider | null) => void;
}

const STATUS_LABEL: Record<ConnectionStatus, string> = {
  [ConnectionStatus.DISCONNECTED]: "Disconnected",
  [ConnectionStatus.CONNECTING]: "Connecting to relay…",
  [ConnectionStatus.WAITING]: "Waiting for wallet scan…",
  [ConnectionStatus.KEY_EXCHANGE]: "Exchanging keys…",
  [ConnectionStatus.CONNECTED]: "Connected",
  [ConnectionStatus.RECONNECTING]: "Reconnecting…",
};

/**
 * Pair-with-wallet button used by the Write contract tab. On click, opens a
 * modal with a QR code + deep-link URI from `@qrlwallet/connect`. After a
 * successful handshake the wallet's address is surfaced upward via
 * `onAccount`. Closing the modal keeps the session active. The explicit
 * Disconnect button terminates it.
 *
 * The QRLConnect instance is a module-level singleton (see
 * lib/qrlConnect.ts) so multiple components sharing it don't double-announce
 * via EIP-6963 or stomp on the persisted-session storage key.
 */
// Lazy snapshot of any stored session so the initial render shows the
// connected pill instead of flashing the "Connect Wallet" CTA before the
// reconnect effect runs. Returning these from useState initializers (rather
// than calling setState() inside useEffect) satisfies the new
// `react-hooks/set-state-in-effect` rule.
function initialAccount(): string | null {
  if (typeof window === "undefined") return null;
  try {
    return getRestorableWalletSession(getQrlConnect()).account;
  } catch {
    // SSR / test environments without window.crypto etc.
  }
  return null;
}
function initialStatus(): ConnectionStatus {
  if (typeof window === "undefined") return ConnectionStatus.DISCONNECTED;
  try {
    const qrl = getQrlConnect();
    if (qrl.hasStoredSession()) return ConnectionStatus.RECONNECTING;
  } catch {
    // Fall through to disconnected.
  }
  return ConnectionStatus.DISCONNECTED;
}

export default function ConnectButton({
  onAccount,
  onProvider,
}: ConnectButtonProps): JSX.Element {
  const [open, setOpen] = useState(false);
  const [uri, setUri] = useState<string | null>(null);
  const [status, setStatus] = useState<ConnectionStatus>(initialStatus);
  const [account, setAccount] = useState<string | null>(initialAccount);
  const [error, setError] = useState<string | null>(null);
  const [disconnecting, setDisconnecting] = useState(false);
  const providerRef = useRef<QRLConnectProvider | null>(null);
  const authorizationRef = useRef<Promise<void> | null>(null);

  // Event wire-up. React runs cleanup before reattaching this effect, including
  // its development Strict Mode cycle, so each event has one active listener.
  // State writes happen inside event-emitter callbacks.
  useEffect(() => {
    let active = true;
    const qrl = getQrlConnect();
    providerRef.current = qrl;
    onProvider?.(qrl);
    // Notify the parent once on mount if we already restored an account
    // from the stored session via the useState initializer above.
    if (account) onAccount?.(account);

    const authorizeAccount = () => {
      if (authorizationRef.current) return;

      const authorization = requestAuthorizedQrlAccount(qrl)
        .then((next) => {
          if (!active) return;
          setAccount(next);
          onAccount?.(next);
          setError(null);
          setOpen(false);
          setUri(null);
        })
        .catch(async (e) => {
          const authorizationError =
            e instanceof Error
              ? e.message
              : "Wallet account authorization failed";
          if (active) {
            setAccount(null);
            onAccount?.(null);
            setError(authorizationError);
            setUri(null);
          }
          try {
            await qrl.disconnect();
          } catch (retirementError) {
            if (!active) return;
            const detail =
              retirementError instanceof Error
                ? retirementError.message
                : "relay channel retirement failed";
            setError(
              `${authorizationError}. The relay session is still active: ${detail}`,
            );
          }
        })
        .finally(() => {
          authorizationRef.current = null;
        });
      authorizationRef.current = authorization;
    };
    const onConnect = () => {
      setStatus(ConnectionStatus.CONNECTED);
      void authorizeAccount();
    };
    const onAccountsChanged = (accounts: string[]) => {
      if (accounts.length === 0) {
        setAccount(null);
        onAccount?.(null);
        return;
      }
      try {
        const next = parseAuthorizedQrlAccount(accounts);
        setAccount(next);
        onAccount?.(next);
        setError(null);
      } catch (e) {
        setAccount(null);
        onAccount?.(null);
        setError(
          e instanceof Error ? e.message : "Wallet returned an invalid account",
        );
      }
    };
    const onDisconnect = () => {
      const stored = getRestorableWalletSession(qrl);
      if (stored.restorable) {
        setStatus(ConnectionStatus.RECONNECTING);
        setAccount(stored.account);
        onAccount?.(stored.account);
        return;
      }
      setStatus(ConnectionStatus.DISCONNECTED);
      setAccount(null);
      onAccount?.(null);
    };
    const onStatusChanged = (s: ConnectionStatus) => setStatus(s);

    qrl.on("connect", onConnect);
    qrl.on("accountsChanged", onAccountsChanged);
    qrl.on("disconnect", onDisconnect);
    qrl.on("statusChanged", onStatusChanged);

    return () => {
      active = false;
      qrl.off("connect", onConnect);
      qrl.off("accountsChanged", onAccountsChanged);
      qrl.off("disconnect", onDisconnect);
      qrl.off("statusChanged", onStatusChanged);
    };
    // `account` is read once at mount only, listing it would re-attach
    // listeners on every account change, which is wrong.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [onAccount, onProvider]);

  const openPairing = useCallback(async () => {
    const qrl = providerRef.current;
    if (!qrl) return;
    setError(null);
    setOpen(true);
    try {
      const next = await qrl.getConnectionURI();
      setUri(next);
      // Mobile devices: jump straight into the wallet via deep link. The
      // QR modal remains rendered as a fallback if the launch fails.
      if (qrl.isMobile()) {
        window.location.href = next;
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }, []);

  const newConnection = useCallback(async () => {
    const qrl = providerRef.current;
    if (!qrl) return;
    setError(null);
    try {
      const next = await qrl.newConnection();
      setUri(next);
      setStatus(ConnectionStatus.WAITING);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }, []);

  const disconnect = useCallback(async () => {
    const qrl = providerRef.current;
    if (!qrl) return;
    setError(null);
    try {
      await qrl.disconnect();
      setOpen(false);
      setUri(null);
    } catch (e) {
      setError(
        e instanceof Error ? e.message : "Unable to retire the relay session",
      );
    }
  }, []);

  const cancelPairing = useCallback(async () => {
    const qrl = providerRef.current;
    if (!qrl || disconnecting) return;
    setError(null);
    setDisconnecting(true);
    try {
      await qrl.disconnect();
      setOpen(false);
      setUri(null);
    } catch (e) {
      setError(
        e instanceof Error ? e.message : "Unable to retire the relay session",
      );
    } finally {
      setDisconnecting(false);
    }
  }, [disconnecting]);

  if (account) {
    const short = `${account.slice(0, 8)}…${account.slice(-6)}`;
    return (
      <div className="space-y-2">
        <div className="flex items-center gap-2 flex-wrap">
          <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full bg-green-500/15 text-green-300 text-xs font-mono">
            <span className="h-1.5 w-1.5 rounded-full bg-green-400" /> {short}
          </span>
          <button
            type="button"
            onClick={disconnect}
            className="inline-flex items-center justify-center px-3 py-1.5 rounded-lg bg-card-gradient border border-border hover:border-accent text-xs text-text-secondary hover:text-accent transition-colors"
          >
            Disconnect
          </button>
        </div>
        {error && <p className="text-xs text-red-300">{error}</p>}
      </div>
    );
  }

  return (
    <>
      <button
        type="button"
        onClick={openPairing}
        className="inline-flex items-center justify-center px-3 py-1.5 rounded-lg bg-accent text-background text-xs font-medium hover:bg-accent-hover transition-colors"
      >
        Connect Wallet
      </button>

      {open && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4"
          onClick={() => void cancelPairing()}
          role="dialog"
          aria-modal="true"
        >
          <div
            className="max-w-sm w-full rounded-xl border border-border bg-card-gradient p-4 md:p-5 space-y-3"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-center justify-between">
              <h3 className="text-sm md:text-base font-semibold text-text-primary">
                Pair MyQRLWallet
              </h3>
              <button
                type="button"
                onClick={() => void cancelPairing()}
                disabled={disconnecting}
                className="text-xs text-text-secondary hover:text-text-primary"
                aria-label="Close"
              >
                {disconnecting ? "Disconnecting…" : "✕"}
              </button>
            </div>

            <p className="text-xs text-text-secondary">
              Scan the QR with MyQRLWallet (mobile) or tap the URI to deep-link
              if you&apos;re on a phone.
            </p>

            <div className="flex justify-center">
              {uri ? (
                <div className="rounded-md bg-white p-3">
                  <QRCodeCanvas value={uri} size={224} level="M" />
                </div>
              ) : (
                <div className="h-[248px] w-[248px] flex items-center justify-center text-xs text-text-muted">
                  Generating QR…
                </div>
              )}
            </div>

            <div className="text-[10px] text-text-muted font-mono break-all">
              {uri ?? ""}
            </div>

            <div className="flex items-center justify-between text-xs">
              <span className="text-text-secondary">
                {STATUS_LABEL[status]}
              </span>
              <button
                type="button"
                onClick={newConnection}
                className="text-accent hover:text-accent-hover"
              >
                New connection
              </button>
            </div>

            {error && (
              <div className="rounded-md border border-red-500/40 bg-red-500/10 p-2 text-xs text-red-300">
                {error}
              </div>
            )}
          </div>
        </div>
      )}
    </>
  );
}
