'use client';

import Link from 'next/link';
import type { ChainTiming } from '../lib/chainTiming';
import {
  activityTimestamp,
  CHANGE_KIND_LABELS,
  type SlashingEvent,
  type ValidatorSetChange,
} from '../lib/validatorActivity';
import TimeDisplay from '../components/TimeDisplay';
import Badge from '../components/Badge';

const headingClass =
  'text-left px-4 py-3 text-[11px] font-normal text-text-muted uppercase tracking-wider whitespace-nowrap';
const rowClass = 'border-b border-border last:border-b-0 hover:bg-surface transition-colors';
const linkClass = 'text-accent hover:text-accent-hover hover:underline tabular-nums';

export function SlashingEventsTable({
  events,
  emptyMessage,
  timing,
}: {
  events: SlashingEvent[];
  emptyMessage: string;
  timing?: ChainTiming;
}) {
  return (
    <div className="card-simple overflow-hidden">
      <div className="overflow-x-auto">
        <table className="w-full text-sm" aria-label="Slashing events">
          <thead>
            <tr>
              {[
                'Slot',
                'Epoch',
                'Time',
                'Type',
                'Slashed validators',
                'Whistleblower',
                'Evidence',
              ].map((label) => (
                <th key={label} scope="col" className={headingClass}>
                  {label}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {events.map((event) => (
              <tr key={event.id} id={`event-${event.id}`} className={rowClass}>
                <td className="px-4 py-3 tabular-nums">{event.slot.toLocaleString()}</td>
                <td className="px-4 py-3">
                  <Link href={`/epoch/${event.epoch}`} className={linkClass}>
                    {event.epoch.toLocaleString()}
                  </Link>
                </td>
                <td className="px-4 py-3 text-text-secondary whitespace-nowrap">
                  <TimeDisplay timestamp={activityTimestamp(event.slot, 'slot', timing)} relative />
                </td>
                <td className="px-4 py-3">
                  <Badge variant="error">
                    {event.type === 'proposer' ? 'Proposer' : 'Attester'}
                  </Badge>
                </td>
                <td className="px-4 py-3">
                  <div className="flex flex-wrap gap-2 min-w-24">
                    {event.validatorIndices.map((index) => (
                      <Link key={index} href={`/validators/${index}`} className={linkClass}>
                        #{index}
                      </Link>
                    ))}
                  </div>
                </td>
                <td className="px-4 py-3">
                  <Link href={`/validators/${event.whistleblower}`} className={linkClass}>
                    #{event.whistleblower}
                  </Link>
                </td>
                <td className="px-4 py-3">
                  <details className="min-w-32 max-w-72">
                    <summary className="cursor-pointer text-accent">Conflicting roots</summary>
                    <div className="mt-2 space-y-2">
                      {event.conflictingRoots.map((root, i) => (
                        <div key={root}>
                          <span className="text-xs text-text-muted">
                            {event.type === 'proposer' ? 'Header' : 'Attestation'} {i + 1}
                          </span>
                          <code className="block font-mono text-xs break-all text-text-secondary">
                            {root}
                          </code>
                        </div>
                      ))}
                    </div>
                  </details>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {events.length === 0 && (
        <p className="px-4 py-12 text-center text-text-muted" role="status">
          {emptyMessage}
        </p>
      )}
    </div>
  );
}

export function ValidatorSetChangesTable({
  changes,
  timing,
  emptyMessage,
}: {
  changes: ValidatorSetChange[];
  timing?: ChainTiming;
  emptyMessage: string;
}) {
  return (
    <div className="card-simple overflow-hidden">
      <div className="overflow-x-auto">
        <table className="w-full text-sm" aria-label="Validator set changes">
          <thead>
            <tr>
              {['Epoch', 'Time', 'Change', 'Validator', 'Public key'].map((label) => (
                <th key={label} scope="col" className={headingClass}>
                  {label}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {changes.map((change) => (
              <tr key={change.id} className={rowClass}>
                <td className="px-4 py-3">
                  <Link href={`/epoch/${change.epoch}`} className={linkClass}>
                    {change.epoch.toLocaleString()}
                  </Link>
                </td>
                <td className="px-4 py-3 text-text-secondary whitespace-nowrap">
                  <TimeDisplay
                    timestamp={activityTimestamp(change.epoch, 'epoch', timing)}
                    relative
                  />
                </td>
                <td className="px-4 py-3">
                  <Badge
                    variant={
                      change.kind === 'slashed'
                        ? 'error'
                        : change.kind === 'activated'
                          ? 'success'
                          : 'neutral'
                    }
                  >
                    {CHANGE_KIND_LABELS[change.kind]}
                  </Badge>
                </td>
                <td className="px-4 py-3">
                  <Link href={`/validators/${change.validatorIndex}`} className={linkClass}>
                    #{change.validatorIndex}
                  </Link>
                </td>
                <td className="px-4 py-3">
                  <Link
                    href={`/validators/${change.validatorIndex}`}
                    className={`${linkClass} font-mono`}
                    aria-label={`Public key for validator ${change.validatorIndex}`}
                  >
                    {change.pubkey.slice(0, 14)}...{change.pubkey.slice(-8)}
                  </Link>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {changes.length === 0 && (
        <p className="px-4 py-12 text-center text-text-muted" role="status">
          {emptyMessage}
        </p>
      )}
    </div>
  );
}
