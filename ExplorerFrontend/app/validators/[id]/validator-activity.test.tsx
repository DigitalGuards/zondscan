import { renderToStaticMarkup } from 'react-dom/server';
import ValidatorActivity from './validator-activity';

jest.mock('../../components/TimeDisplay', () => ({
  __esModule: true,
  default: () => null,
}));

it('shows an unslashed validator and a link to its filtered events', () => {
  const html = renderToStaticMarkup(
    <ValidatorActivity index="0" slashed={false} events={[]} changes={[]} />,
  );
  expect(html).toContain('Not slashed');
  expect(html).toContain('This validator has no slashing events.');
  expect(html).toContain('No set changes have been indexed for this validator yet.');
  expect(html).not.toContain('match these filters');
  expect(html).toContain('href="/slashings?validator=0"');
});

it('preserves slashed status when evidence has not been indexed', () => {
  const html = renderToStaticMarkup(
    <ValidatorActivity index="0" slashed events={[]} changes={[]} />,
  );
  expect(html).toContain('>Slashed</span>');
  expect(html).toContain('Slashing evidence for this validator has not been indexed.');
});

it('shows the validator slashing event and its including proposer', () => {
  const html = renderToStaticMarkup(
    <ValidatorActivity
      index="0"
      slashed
      events={[
        {
          id: '17:proposer:0',
          slot: 17,
          epoch: 2,
          type: 'proposer',
          validatorIndices: ['0'],
          whistleblower: '4',
          conflictingRoots: ['0x' + 'a'.repeat(64), '0x' + 'b'.repeat(64)],
        },
      ]}
      changes={[
        {
          id: '3:slashed:0',
          epoch: 3,
          kind: 'slashed',
          validatorIndex: '0',
          pubkey: '0xaabb',
        },
      ]}
    />,
  );
  expect(html).toContain('Proposer');
  expect(html).toContain('href="/validators/4"');
  expect(html).toContain('href="/epoch/3"');
});
