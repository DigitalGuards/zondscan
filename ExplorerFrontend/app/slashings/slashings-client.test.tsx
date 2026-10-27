import { renderToStaticMarkup } from 'react-dom/server';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import SlashingsClient from './slashings-client';
import SlashingsPage from './page';
import {
  parseSlashings,
  parseValidatorSetChanges,
  slashingEmptyMessage,
  activityTimestamp,
  type SlashingsData,
  type ValidatorSetChangesData,
} from '../lib/validatorActivity';
import { NAVIGATION_GROUPS } from '../lib/navigation';

jest.mock('../lib/useChainTiming', () => ({
  useChainTiming: () => ({
    data: { slotsPerEpoch: 8, secondsPerSlot: 60, genesisTime: 1000 },
  }),
}));
jest.mock('../components/TimeDisplay', () => ({
  __esModule: true,
  default: ({ timestamp }: { timestamp: number | null }) => <span>{timestamp ?? 'Unknown'}</span>,
}));

const emptyEvents: SlashingsData = {
  events: [],
  total: 0,
  page: 1,
  limit: 20,
  slashedValidatorCount: 0,
  backfillComplete: true,
};
const emptyChanges: ValidatorSetChangesData = {
  changes: [],
  total: 0,
  page: 1,
  limit: 20,
  backfillComplete: true,
};

function render(events = emptyEvents, changes = emptyChanges, validatorIndex?: string) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  client.setQueryData(['slashings', validatorIndex, 1], events);
  client.setQueryData(
    ['validator-set-changes', validatorIndex, 1, { kind: '', fromEpoch: '', toEpoch: '' }],
    changes,
  );
  const html = renderToStaticMarkup(
    <QueryClientProvider client={client}>
      <SlashingsClient validatorIndex={validatorIndex} />
    </QueryClientProvider>,
  );
  client.clear();
  return html;
}

it('shows the plain network empty state and both consensus tables', () => {
  const html = render();
  expect(html).toContain('No validator has been slashed on this network yet.');
  expect(html).toContain('aria-label="Slashing events"');
  expect(html).toContain('aria-label="Validator set changes"');
  expect(html).toContain('No validator set changes match these filters.');
  expect(html).toContain('<option value="ejected">Ejected (low balance)</option>');
  expect(html).toContain('effective balance at exit is at or below the network threshold.');
  expect(NAVIGATION_GROUPS.find((group) => group.name === 'Consensus')?.items).toContainEqual(
    expect.objectContaining({ name: 'Slashings', href: '/slashings' }),
  );
});

it.each([
  ['007', '7'],
  ['000', '0'],
  ['0', '0'],
])('canonicalizes validator %s before rendering links', async (validator, canonical) => {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const page = await SlashingsPage({
    searchParams: Promise.resolve({ validator }),
  });
  const html = renderToStaticMarkup(
    <QueryClientProvider client={client}>{page}</QueryClientProvider>,
  );
  client.clear();
  expect(html).toContain(`href="/validators/${canonical}"`);
  expect(html).toContain(`>#${canonical}</a>`);
  if (validator !== canonical) expect(html).not.toContain(`/validators/${validator}`);
});

it('renders evidence, every change kind, validator zero links and network times', () => {
  const events = parseSlashings({
    ...emptyEvents,
    total: 21,
    slashedValidatorCount: 2,
    events: [
      {
        id: '17:attester:0',
        slot: 17,
        epoch: 2,
        type: 'attester',
        validatorIndices: ['0', '3'],
        whistleblower: '4',
        conflictingRoots: ['0x' + 'a'.repeat(64), '0x' + 'b'.repeat(64)],
      },
    ],
  });
  const changes = parseValidatorSetChanges({
    ...emptyChanges,
    total: 4,
    changes: ['activated', 'exited', 'slashed', 'ejected'].map((kind) => ({
      id: `2:${kind}:0`,
      epoch: 2,
      kind,
      validatorIndex: '0',
      pubkey: '0x' + 'c'.repeat(64),
    })),
  });
  const html = render(events, changes);
  for (const text of [
    'Attester',
    'Activated',
    'Exited',
    'Slashed',
    'Ejected (low balance)',
    'Conflicting roots',
    '0x' + 'a'.repeat(64),
  ])
    expect(html).toContain(text);
  for (const href of ['/validators/0', '/validators/3', '/validators/4', '/epoch/2'])
    expect(html).toContain(`href="${href}"`);
  expect(html).toContain('>2020</span>');
  expect(html).toContain('>1960</span>');
  expect(html).toContain('Page 1 of 2');
  expect(html).not.toContain('No validator has been slashed');
});

it('keeps validator filters and incomplete evidence distinct from the network empty state', () => {
  expect(render(emptyEvents, emptyChanges, '0')).toContain(
    'No slashing events have been indexed for this validator.',
  );
  for (const backfillComplete of [false, true]) {
    expect(
      render({ ...emptyEvents, slashedValidatorCount: 1, backfillComplete }),
    ).toContain(
      'Slashed validators have been recorded. Their slashing evidence has not been indexed.',
    );
  }
  expect(
    render({
      ...emptyEvents,
      slashedValidatorCount: 1,
      backfillComplete: false,
    }),
  ).toContain('Historical validator activity is still syncing.');
  expect(slashingEmptyMessage({ ...emptyEvents, total: 1 })).toBe(
    'No slashing events on this page.',
  );
});

it('rejects malformed payloads and keeps unknown timing unavailable', () => {
  expect(() => parseSlashings({ ...emptyEvents, events: null })).toThrow();
  expect(() => parseSlashings({ ...emptyEvents, events: [{ id: 'bad' }] })).toThrow();
  expect(() =>
    parseValidatorSetChanges({
      ...emptyChanges,
      changes: [{ epoch: 1, kind: 'unknown' }],
    }),
  ).toThrow();
  expect(activityTimestamp(2, 'epoch')).toBeNull();
  expect(
    activityTimestamp(2, 'epoch', {
      slotsPerEpoch: 4,
      secondsPerSlot: 12,
      genesisTime: 1000,
    }),
  ).toBe(1096);
});
