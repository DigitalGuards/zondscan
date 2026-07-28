'use client';

import { useEffect, useMemo, useState } from 'react';
import type { FormEvent } from 'react';
import axios, { AxiosError } from 'axios';
import { useSearchParams } from 'next/navigation';
import Link from 'next/link';
import config from '../../config';

interface CompilerBuild {
  buildId: string;
  language: string;
  default?: boolean;
}

interface CompilerInfo {
  language: string;
  buildId: string;
  default?: string;
  compilers?: CompilerBuild[];
}

interface VerifyEnqueueResponse {
  jobId: string;
  status: string;
  address: string;
  alreadyVerified?: boolean;
}

interface VerificationJob {
  jobId: string;
  status: 'pending' | 'compiling' | 'success' | 'failed';
  error?: string;
  address: string;
  payload?: {
    contractName?: string;
    compilerVersion?: string;
  };
}

const COMMON_LICENSES = ['MIT', 'GPL-3.0', 'Apache-2.0', 'BSD-2-Clause', 'BSD-3-Clause', 'Unlicense', 'No license'];

/**
 * Client-side verify-contract form. Posts to /contract/verify and polls
 * /contract/verify/:jobId until terminal. The compiler-version selector is
 * populated from /contract/compiler-info; the user picks which configured
 * Hyperion build to compile against (defaulting to the backend's default
 * build). The byte-match only succeeds against the exact build the contract
 * was deployed with, so offering the real list is what lets authors verify.
 */
export default function VerifyContractClient(): JSX.Element {
  const searchParams = useSearchParams();
  const prefillAddress = searchParams?.get('address') ?? '';

  const [address, setAddress] = useState(prefillAddress);
  const [contractName, setContractName] = useState('');
  const [sourceCode, setSourceCode] = useState('');
  const [importsText, setImportsText] = useState('');
  const [optimizerEnabled, setOptimizerEnabled] = useState(false);
  const [optimizerRuns, setOptimizerRuns] = useState(200);
  const [evmVersion, setEvmVersion] = useState('');
  const [constructorArgs, setConstructorArgs] = useState('');
  const [license, setLicense] = useState('MIT');

  const [compilerInfo, setCompilerInfo] = useState<CompilerInfo | null>(null);
  const [compilerErr, setCompilerErr] = useState<string | null>(null);
  const [selectedBuild, setSelectedBuild] = useState('');

  const [submitting, setSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [job, setJob] = useState<VerificationJob | null>(null);
  const [alreadyVerified, setAlreadyVerified] = useState<string | null>(null);

  // Fetch the available compiler builds once on mount and preselect the
  // backend's default. If the backend doesn't expose them (503), surface
  // that as a banner, submitting in that state will just 503 from the
  // backend.
  useEffect(() => {
    let cancelled = false;
    axios.get<CompilerInfo>(`${config.handlerUrl}/contract/compiler-info`)
      .then(r => {
        if (cancelled) return;
        setCompilerInfo(r.data);
        setSelectedBuild(r.data.default || r.data.buildId || '');
      })
      .catch(e => { if (!cancelled) setCompilerErr(extractAxiosError(e)); });
    return () => { cancelled = true; };
  }, []);

  // Available builds, tolerant of an older single-build backend that only
  // returns { language, buildId } (no `compilers` array).
  const compilerBuilds: CompilerBuild[] = useMemo(() => {
    if (!compilerInfo) return [];
    if (compilerInfo.compilers && compilerInfo.compilers.length > 0) {
      return compilerInfo.compilers;
    }
    return [{ buildId: compilerInfo.buildId, language: compilerInfo.language, default: true }];
  }, [compilerInfo]);

  const isTerminal = job?.status === 'success' || job?.status === 'failed';

  // Poll the job to terminal. Depend on the stable jobId (not the whole
  // job object) so setJob() inside the poller doesn't restart the timer
  //, without this the effective cadence drifts between 1.5–3s as the
  // interval re-arms after every response.
  const jobId = job?.jobId;
  useEffect(() => {
    if (!jobId || isTerminal) return;
    let cancelled = false;
    const id = setInterval(async () => {
      try {
        const r = await axios.get<VerificationJob>(`${config.handlerUrl}/contract/verify/${jobId}`);
        if (cancelled) return;
        setJob(r.data);
      } catch (e) {
        if (cancelled) return;
        setJob(j => j ? { ...j, status: 'failed', error: extractAxiosError(e) } : j);
      }
    }, 1500);
    return () => { cancelled = true; clearInterval(id); };
  }, [jobId, isTerminal]);

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setSubmitError(null);
    setAlreadyVerified(null);
    setJob(null);

    let imports: Record<string, string> | undefined;
    if (importsText.trim()) {
      try {
        const parsed = JSON.parse(importsText) as unknown;
        if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
          imports = parsed as Record<string, string>;
        } else {
          setSubmitError('Imports must be a JSON object: { "Filename.hyp": "<source>" }');
          return;
        }
      } catch (e) {
        setSubmitError('Imports field is not valid JSON: ' + (e instanceof Error ? e.message : String(e)));
        return;
      }
    }

    setSubmitting(true);
    try {
      const resp = await axios.post<VerifyEnqueueResponse>(`${config.handlerUrl}/contract/verify`, {
        address: address.trim(),
        sourceCode,
        contractName: contractName.trim(),
        compilerVersion: selectedBuild || undefined,
        optimizerEnabled,
        // Pass the raw optimizerRuns through, 0 is a legitimate
        // configuration distinct from the default, so don't fall back
        // via `|| 200`.
        optimizerRuns: optimizerRuns,
        evmVersion: evmVersion.trim() || undefined,
        constructorArguments: constructorArgs.trim() || undefined,
        imports,
        license: license || undefined,
      });
      if (resp.data.alreadyVerified) {
        setAlreadyVerified(resp.data.address);
      } else {
        setJob({ jobId: resp.data.jobId, status: 'pending', address: resp.data.address });
      }
    } catch (e) {
      setSubmitError(extractAxiosError(e));
    } finally {
      setSubmitting(false);
    }
  };

  const compilerBanner = useMemo(() => {
    if (compilerErr) {
      return (
        <div className="rounded-lg border border-red-500/40 bg-red-500/10 text-red-300 text-xs md:text-sm p-3">
          Verification is not available right now (compiler-info: {compilerErr}).
        </div>
      );
    }
    return null;
  }, [compilerErr]);

  return (
    <div className="space-y-3 md:space-y-4">
      {compilerBanner}

      {alreadyVerified && (
        <div className="rounded-lg border border-green-500/40 bg-green-500/10 text-green-300 text-xs md:text-sm p-3">
          This contract is already verified.{' '}
          <Link href={`/address/${alreadyVerified}`} className="underline">Go to address page →</Link>
        </div>
      )}

      {job && (
        <div className={`rounded-lg border p-3 text-xs md:text-sm ${
          job.status === 'success' ? 'border-green-500/40 bg-green-500/10 text-green-300' :
          job.status === 'failed' ? 'border-red-500/40 bg-red-500/10 text-red-300' :
          'border-border bg-card-gradient text-text-secondary'
        }`}>
          <div className="font-medium">
            {job.status === 'pending' && 'Queued…'}
            {job.status === 'compiling' && 'Compiling + matching bytecode…'}
            {job.status === 'success' && 'Verified ✓'}
            {job.status === 'failed' && 'Verification failed'}
          </div>
          {job.status === 'success' && (
            <div className="mt-1">
              <Link href={`/address/${job.address}`} className="underline">View verified contract →</Link>
            </div>
          )}
          {job.error && (
            <pre className="mt-2 whitespace-pre-wrap break-words font-mono text-[11px] opacity-80">
              {job.error}
            </pre>
          )}
        </div>
      )}

      {submitError && (
        <div className="rounded-lg border border-red-500/40 bg-red-500/10 text-red-300 text-xs md:text-sm p-3">
          {submitError}
        </div>
      )}

      <form onSubmit={handleSubmit} className="space-y-3 md:space-y-4">
        {compilerBuilds.length > 0 && (
          <Field
            label="Compiler"
            hint="Pick the Hyperion build your contract was compiled with - the byte-match only succeeds against that exact build."
          >
            {compilerBuilds.length === 1 ? (
              <div className="form-input font-mono text-xs break-all bg-transparent">
                {compilerBuilds[0].language} {compilerBuilds[0].buildId}
              </div>
            ) : (
              <select
                value={selectedBuild}
                onChange={e => setSelectedBuild(e.target.value)}
                disabled={submitting}
                className="form-input font-mono text-xs"
              >
                {compilerBuilds.map(b => (
                  <option key={b.buildId} value={b.buildId}>
                    {b.language} {b.buildId}{b.default ? ' (default)' : ''}
                  </option>
                ))}
              </select>
            )}
          </Field>
        )}

        <Field label="Contract address (Q…)">
          <input
            required
            value={address}
            onChange={e => setAddress(e.target.value)}
            placeholder="Q…"
            className="form-input font-mono"
          />
        </Field>

        <Field label="Contract name" hint="Must match a contract declared in the source.">
          <input
            required
            value={contractName}
            onChange={e => setContractName(e.target.value)}
            placeholder="e.g. SimpleERC721"
            className="form-input"
          />
        </Field>

        <div className="grid grid-cols-1 md:grid-cols-3 gap-3 md:gap-4">
          <Field label="Optimizer">
            <label className="flex items-center gap-2 text-sm text-text-primary">
              <input
                type="checkbox"
                checked={optimizerEnabled}
                onChange={e => setOptimizerEnabled(e.target.checked)}
                className="form-checkbox"
              />
              Enabled
            </label>
          </Field>
          <Field label="Optimizer runs">
            <input
              type="number"
              min={0}
              value={optimizerRuns}
              onChange={e => setOptimizerRuns(Number(e.target.value))}
              disabled={!optimizerEnabled}
              className="form-input"
            />
          </Field>
          <Field label="License">
            <select
              value={license}
              onChange={e => setLicense(e.target.value)}
              className="form-input"
            >
              {COMMON_LICENSES.map(l => <option key={l} value={l}>{l}</option>)}
            </select>
          </Field>
        </div>

        <Field label="EVM version (optional)">
          <input
            value={evmVersion}
            onChange={e => setEvmVersion(e.target.value)}
            placeholder="e.g. shanghai"
            className="form-input"
          />
        </Field>

        <Field label="Source code (single file)" hint="Submit the contract source as a single .hyp file. Multi-file imports go in the Imports field below.">
          <textarea
            required
            value={sourceCode}
            onChange={e => setSourceCode(e.target.value)}
            rows={14}
            placeholder="// SPDX-License-Identifier: MIT&#10;pragma hyperion ^0.0.2;&#10;contract MyContract { … }"
            className="form-input font-mono text-xs"
          />
        </Field>

        <Field
          label="Imports (optional)"
          hint='JSON object mapping import paths → source contents. Example: {"Context.hyp": "...source..."}'
        >
          <textarea
            value={importsText}
            onChange={e => setImportsText(e.target.value)}
            rows={6}
            placeholder='{"Context.hyp": "// SPDX-License-Identifier: MIT&#10;pragma hyperion ^0.0.2;&#10;abstract contract Context { … }"}'
            className="form-input font-mono text-xs"
          />
        </Field>

        <Field label="Constructor arguments (optional)" hint="Hex-encoded ABI tuple. Advisory, not used for the byte-match check.">
          <input
            value={constructorArgs}
            onChange={e => setConstructorArgs(e.target.value)}
            placeholder="0x…"
            className="form-input font-mono text-xs"
          />
        </Field>

        <button
          type="submit"
          disabled={submitting || (job?.status === 'pending' || job?.status === 'compiling')}
          className="inline-flex items-center justify-center px-4 py-2 rounded-lg bg-accent text-background text-sm font-medium hover:bg-accent-hover transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {submitting ? 'Submitting…' : 'Verify & Publish'}
        </button>
      </form>
    </div>
  );
}

function Field({ label, hint, children }: { label: string; hint?: string; children: React.ReactNode }) {
  return (
    <div>
      <div className="text-xs md:text-sm text-text-secondary mb-1">{label}</div>
      {children}
      {hint && <div className="text-[11px] text-text-muted mt-1">{hint}</div>}
    </div>
  );
}

function extractAxiosError(e: unknown): string {
  if (e instanceof AxiosError) {
    const d = e.response?.data as { error?: string; message?: string } | undefined;
    return d?.error || d?.message || e.message;
  }
  return e instanceof Error ? e.message : String(e);
}
