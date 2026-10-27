'use client';

import Link from 'next/link';
import Badge from '../../components/Badge';
import type { ChainTiming } from '../../lib/chainTiming';
import type { SlashingEvent, ValidatorSetChange } from '../../lib/validatorActivity';
import { SlashingEventsTable, ValidatorSetChangesTable } from '../../slashings/activity-tables';

export default function ValidatorActivity({
  index,
  slashed,
  events,
  changes,
  timing,
}: {
  index: string;
  slashed: boolean;
  events?: SlashingEvent[];
  changes?: ValidatorSetChange[];
  timing?: ChainTiming;
}) {
  return (
    <section aria-labelledby="validator-activity-heading" className="mt-6 space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2
          id="validator-activity-heading"
          className="font-display text-lg font-semibold text-text-primary"
        >
          Slashing status and events
        </h2>
        <Badge variant={slashed ? 'error' : 'success'}>{slashed ? 'Slashed' : 'Not slashed'}</Badge>
      </div>
      {events && changes ? (
        <>
          <SlashingEventsTable
            events={events}
            emptyMessage={
              slashed
                ? 'Slashing evidence for this validator has not been indexed.'
                : 'This validator has no slashing events.'
            }
            timing={timing}
          />
          <h3 className="font-display font-semibold text-text-primary">Validator set changes</h3>
          <ValidatorSetChangesTable
            changes={changes}
            timing={timing}
            emptyMessage="No set changes have been indexed for this validator yet."
          />
        </>
      ) : (
        <p role="alert" className="text-error">
          Slashing activity could not be loaded.
        </p>
      )}
      <Link
        href={`/slashings?validator=${index}`}
        className="inline-block text-sm text-accent hover:underline"
      >
        View all events for validator #{index}
      </Link>
    </section>
  );
}
