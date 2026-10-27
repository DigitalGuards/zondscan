import { renderToStaticMarkup } from 'react-dom/server';
import { parseValidatorDetail, ValidatorDetailContent } from './validator-detail-client';

jest.mock('../../components/TimeDisplay', () => ({
  __esModule: true,
  default: () => null,
}));

const validator = {
  index: '7',
  publicKeyHex: 'aabb',
  withdrawalCredentialsHex: '00'.repeat(32),
  effectiveBalance: '40000000000000',
  slashed: false,
  activationEligibilityEpoch: '0',
  activationEpoch: '1',
  exitEpoch: '18446744073709551615',
  withdrawableEpoch: '18446744073709551615',
  status: 'active',
  age: 12,
  currentEpoch: '13',
};

it.each([
  ['missing activity', {}],
  ['missing changes', { slashingEvents: [] }],
  ['missing events', { validatorSetChanges: [] }],
  ['malformed events', { slashingEvents: [{ id: 'odd-row' }], validatorSetChanges: [] }],
  ['malformed changes', { slashingEvents: [], validatorSetChanges: [{ kind: 'odd-row' }] }],
  ['null activity', { slashingEvents: null, validatorSetChanges: null }],
])('renders the validator with an activity warning for %s', (_name, activity) => {
  const parsed = parseValidatorDetail({ ...validator, ...activity });
  expect(parsed.activity).toBeNull();
  const html = renderToStaticMarkup(<ValidatorDetailContent {...parsed} />);
  expect(html).toContain('Validator #7');
  expect(html).toContain('Effective Balance');
  expect(html).toContain('Validator Details');
  expect(html).toContain('Epoch Timeline');
  expect(html).toContain('Slashing activity could not be loaded.');
  expect(html).toContain('Not slashed');
  expect(html).toContain('href="/slashings?validator=7"');
  expect(html).not.toContain('This validator has no slashing events.');
});

it('renders valid empty activity without a warning', () => {
  const parsed = parseValidatorDetail({
    ...validator,
    slashingEvents: [],
    validatorSetChanges: [],
  });
  const html = renderToStaticMarkup(<ValidatorDetailContent {...parsed} />);
  expect(html).toContain('Validator #7');
  expect(html).toContain('This validator has no slashing events.');
  expect(html).not.toContain('Slashing activity could not be loaded.');
});

it.each([
  null,
  { ...validator, slashed: 'false' },
  { ...validator, status: null },
  { ...validator, age: '12' },
])('rejects invalid core validator fields: %p', (data) => {
  expect(() => parseValidatorDetail(data)).toThrow('Invalid validator response');
});
