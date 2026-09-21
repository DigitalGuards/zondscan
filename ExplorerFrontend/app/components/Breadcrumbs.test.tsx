import { renderToStaticMarkup } from 'react-dom/server';
import Breadcrumbs from './Breadcrumbs';

jest.mock('./PreferencesProvider', () => ({
  usePreferences: () => ({ preferences: { locale: 'es' } }),
}));

function visibleText(html: string): string {
  return html.replace(/<[^>]*>/g, '');
}

describe('Breadcrumbs with Spanish preferences', () => {
  it.each(['TIME', 'Home', '__proto__', 'constructor'])(
    'preserves the dynamic label %s in linked and current breadcrumbs',
    (label) => {
      const linked = renderToStaticMarkup(
        <Breadcrumbs
          items={[
            { label, href: '/token' },
            { label: 'Details', translateLabel: true },
          ]}
        />
      );
      const current = renderToStaticMarkup(<Breadcrumbs items={[{ label }]} />);

      expect(visibleText(linked)).toBe(`Inicio${label}Detalles`);
      expect(linked).toContain('href="/token"');
      expect(visibleText(current)).toBe(`Inicio${label}`);
      expect(current).toContain('aria-current="page"');
    }
  );

  it('translates linked and current UI labels when explicitly enabled', () => {
    const html = renderToStaticMarkup(
      <Breadcrumbs
        items={[
          { label: 'Transactions', href: '/transactions/1', translateLabel: true },
          { label: 'Time', translateLabel: true },
        ]}
      />
    );

    expect(visibleText(html)).toBe('InicioTransaccionesHora');
  });

  it('preserves labels when translation is explicitly disabled', () => {
    const html = renderToStaticMarkup(
      <Breadcrumbs items={[{ label: 'Time', translateLabel: false }]} />
    );

    expect(visibleText(html)).toBe('InicioTime');
  });
});
