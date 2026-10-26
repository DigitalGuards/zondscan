import { renderToStaticMarkup } from 'react-dom/server';
import Page from './page';
import { fetchChainTiming } from '../../lib/chainTiming';

jest.mock('../../lib/chainTiming', () => ({ fetchChainTiming: jest.fn() }));
jest.mock('../components/LearnArticle', () => ({
  __esModule: true,
  default: ({ children }: { children: React.ReactNode }) => <article>{children}</article>,
  LearnSection: ({ children }: { children: React.ReactNode }) => <section>{children}</section>,
  StepList: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  Step: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  Callout: ({ children }: { children: React.ReactNode }) => <aside>{children}</aside>,
  LearnFAQ: () => null,
}));

it('explains mainnet parameters and shows this network timing from the API', async () => {
  jest.mocked(fetchChainTiming).mockResolvedValue({ slotsPerEpoch: 8, secondsPerSlot: 60, genesisTime: 1790582760 });
  const html = renderToStaticMarkup(await Page());
  expect(html).toContain('60 seconds per slot');
  expect(html).toContain('8 consecutive slots');
  expect(html).toContain('taking 8 minutes');
  expect(html).toContain('128 consecutive slots');
  expect(html).toContain('taking 128 minutes');
  expect(html).toContain('qrysm mainnet config');
  expect(html).toContain('network uses its own values');
});

it('shows dashes when network timing is unavailable', async () => {
  jest.mocked(fetchChainTiming).mockRejectedValue(new Error('Unavailable'));
  const html = renderToStaticMarkup(await Page());
  expect(html).toContain('- seconds per slot');
  expect(html).toContain('- consecutive slots');
  expect(html).toContain('128 consecutive slots');
  expect(html).toContain('60 seconds per slot');
});
