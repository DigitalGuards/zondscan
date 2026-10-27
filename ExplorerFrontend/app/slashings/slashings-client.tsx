'use client';

import { useState, type SyntheticEvent } from 'react';
import { useQuery } from '@tanstack/react-query';
import Link from 'next/link';
import config from '../../config';
import { useChainTiming } from '../lib/useChainTiming';
import {
  CHANGE_KINDS,
  CHANGE_KIND_LABELS,
  parseSlashings,
  parseValidatorSetChanges,
  slashingEmptyMessage,
} from '../lib/validatorActivity';
import { SlashingEventsTable, ValidatorSetChangesTable } from './activity-tables';
import Pagination from '../components/Pagination';

const PAGE_SIZE = 20;
const fieldClass =
  'rounded-lg border border-border bg-surface px-3 py-2 text-sm text-text-primary min-w-0 w-full';

async function getActivity(
  path: string,
  params: URLSearchParams,
  signal: AbortSignal,
): Promise<unknown> {
  const response = await fetch(`${config.handlerUrl}/${path}?${params.toString()}`, { signal });
  if (!response.ok) throw new Error('Validator activity is unavailable. Please try again.');
  return response.json();
}

export default function SlashingsClient({ validatorIndex }: { validatorIndex?: string }) {
  const [eventPage, setEventPage] = useState(1);
  const [changePage, setChangePage] = useState(1);
  const [filters, setFilters] = useState({
    kind: '',
    fromEpoch: '',
    toEpoch: '',
  });
  const [filterError, setFilterError] = useState('');
  const { data: timing } = useChainTiming();
  const params = (page: number) => {
    const result = new URLSearchParams({
      page: String(page),
      limit: String(PAGE_SIZE),
    });
    if (validatorIndex !== undefined) result.set('validator', validatorIndex);
    return result;
  };

  const events = useQuery({
    queryKey: ['slashings', validatorIndex, eventPage],
    queryFn: async ({ signal }) =>
      parseSlashings(await getActivity('slashings', params(eventPage), signal)),
    refetchInterval: 30_000,
    refetchIntervalInBackground: false,
    staleTime: 30_000,
  });
  const changes = useQuery({
    queryKey: ['validator-set-changes', validatorIndex, changePage, filters],
    queryFn: async ({ signal }) => {
      const query = params(changePage);
      for (const [key, value] of Object.entries(filters)) if (value !== '') query.set(key, value);
      return parseValidatorSetChanges(await getActivity('validator-set-changes', query, signal));
    },
    refetchInterval: 30_000,
    refetchIntervalInBackground: false,
    staleTime: 30_000,
  });

  function applyFilters(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    const textField = (name: string) => {
      const value = form.get(name);
      return typeof value === 'string' ? value.trim() : '';
    };
    const fromEpoch = textField('fromEpoch');
    const toEpoch = textField('toEpoch');
    const kind = textField('kind');
    if (
      [fromEpoch, toEpoch].some(
        (value) => value !== '' && (!/^\d+$/.test(value) || !Number.isSafeInteger(Number(value))),
      ) ||
      (fromEpoch !== '' && toEpoch !== '' && BigInt(fromEpoch) > BigInt(toEpoch))
    ) {
      setFilterError('Enter a valid epoch range with the first epoch at or before the last.');
      return;
    }
    setFilterError('');
    setFilters({ kind, fromEpoch, toEpoch });
    setChangePage(1);
  }

  return (
    <div className="page-content py-4 sm:py-6 lg:py-8 space-y-8">
      <div>
        <h1 className="section-title">Slashings</h1>
        <p className="text-text-secondary mt-2">
          Slashing evidence and changes to the validators securing this network.
        </p>
        {validatorIndex !== undefined && (
          <p className="mt-3 text-sm text-text-secondary">
            Showing validator{' '}
            <Link href={`/validators/${validatorIndex}`} className="text-accent hover:underline">
              #{validatorIndex}
            </Link>
            .{' '}
            <Link href="/slashings" className="text-accent hover:underline">
              View all validators
            </Link>
          </p>
        )}
      </div>

      <section aria-labelledby="slashing-events-heading">
        <h2
          id="slashing-events-heading"
          className="font-display text-lg font-semibold text-text-primary mb-2"
        >
          Slashing events
        </h2>
        <p className="text-sm text-text-muted mb-4">
          The whistleblower is the proposer of the block that included the evidence.
        </p>
        {events.isError ? (
          <p role="alert" className="text-error">
            Slashing events could not be loaded. Please try again.
          </p>
        ) : events.data ? (
          <>
            {!events.data.backfillComplete && (
              <p className="text-sm text-text-muted mb-3">
                Historical validator activity is still syncing.
              </p>
            )}
            <SlashingEventsTable
              events={events.data.events}
              emptyMessage={slashingEmptyMessage(events.data, validatorIndex)}
              timing={timing}
            />
            <Pagination
              currentPage={eventPage}
              totalPages={Math.max(1, Math.ceil(events.data.total / PAGE_SIZE))}
              onPrevious={() => {
                setEventPage((page) => Math.max(1, page - 1));
              }}
              onNext={() => {
                setEventPage((page) => page + 1);
              }}
            />
          </>
        ) : (
          <p role="status" className="text-text-muted py-8">
            Loading slashing events...
          </p>
        )}
      </section>

      <section aria-labelledby="validator-set-changes-heading">
        <h2
          id="validator-set-changes-heading"
          className="font-display text-lg font-semibold text-text-primary mb-2"
        >
          Validator set changes
        </h2>
        <p className="text-sm text-text-muted mb-4">
          Slashed marks the first epoch with a recorded slashing. Ejected marks unslashed
          validators whose effective balance at exit is at or below the network threshold.
        </p>
        <form onSubmit={applyFilters} className="flex flex-wrap items-end gap-3 mb-4">
          <div className="text-sm text-text-secondary flex-1 min-w-40">
            <label htmlFor="change-kind">Change</label>
            <select id="change-kind" name="kind" className={`${fieldClass} mt-1`} defaultValue="">
              <option value="">All changes</option>
              {CHANGE_KINDS.map((kind) => (
                <option key={kind} value={kind}>
                  {CHANGE_KIND_LABELS[kind]}
                </option>
              ))}
            </select>
          </div>
          <label className="text-sm text-text-secondary flex-1 min-w-32">
            From epoch
            <input
              name="fromEpoch"
              inputMode="numeric"
              pattern="[0-9]*"
              className={`${fieldClass} mt-1`}
              placeholder="0"
            />
          </label>
          <label className="text-sm text-text-secondary flex-1 min-w-32">
            To epoch
            <input
              name="toEpoch"
              inputMode="numeric"
              pattern="[0-9]*"
              className={`${fieldClass} mt-1`}
              placeholder="Latest"
            />
          </label>
          <button type="submit" className="btn-secondary text-sm">
            Apply filters
          </button>
        </form>
        {filterError && (
          <p role="alert" className="text-error text-sm mb-3">
            {filterError}
          </p>
        )}
        {changes.isError ? (
          <p role="alert" className="text-error">
            Validator set changes could not be loaded. Please try again.
          </p>
        ) : changes.data ? (
          <>
            {!changes.data.backfillComplete && (
              <p className="text-sm text-text-muted mb-3">
                Historical validator set changes are still syncing.
              </p>
            )}
            <ValidatorSetChangesTable
              changes={changes.data.changes}
              timing={timing}
              emptyMessage="No validator set changes match these filters."
            />
            <Pagination
              currentPage={changePage}
              totalPages={Math.max(1, Math.ceil(changes.data.total / PAGE_SIZE))}
              onPrevious={() => {
                setChangePage((page) => Math.max(1, page - 1));
              }}
              onNext={() => {
                setChangePage((page) => page + 1);
              }}
            />
          </>
        ) : (
          <p role="status" className="text-text-muted py-8">
            Loading validator set changes...
          </p>
        )}
      </section>
    </div>
  );
}
