'use client';

export interface TabPill<T extends string = string> {
  key: T;
  label: string;
  /** Optional count rendered as "(badge)" after the label. */
  badge?: string | null;
}

/**
 * Etherscan-style pill tab bar, shared by the address and token contract
 * pages. Replaces the old underline tabs: an underline indicator sliding
 * over a static border-b inside an overflow-x-auto container read as
 * "broken/draggable" on phones, while a row of filled chips makes the
 * horizontal scroll look intentional. The scrollbar is hidden
 * (scrollbar-none) like Etherscan's mobile tab row.
 *
 * When `withPanelIds` is set, buttons emit the tab-<key> / tabpanel-<key>
 * id pairing the address page's tabpanels reference via aria-labelledby.
 *
 * Generic over the tab key union so callers' onSelect handlers take their
 * own key type directly, no `as` assertions at the call sites.
 */
export default function TabPillBar<T extends string>({
  tabs,
  activeKey,
  onSelect,
  ariaLabel,
  withPanelIds = false,
}: {
  tabs: TabPill<T>[];
  activeKey: T;
  onSelect: (key: T) => void;
  ariaLabel: string;
  withPanelIds?: boolean;
}): JSX.Element {
  return (
    <div
      role="tablist"
      aria-label={ariaLabel}
      className="flex gap-2 overflow-x-auto hide-scrollbar -mx-1 px-1 py-1"
    >
      {tabs.map((tab) => {
        const active = tab.key === activeKey;
        return (
          <button
            key={tab.key}
            id={withPanelIds ? `tab-${tab.key}` : undefined}
            role="tab"
            aria-selected={active}
            aria-controls={withPanelIds ? `tabpanel-${tab.key}` : undefined}
            onClick={() => onSelect(tab.key)}
            type="button"
            className={`whitespace-nowrap shrink-0 rounded-lg px-3 md:px-4 py-1.5 md:py-2 text-xs md:text-sm font-medium border transition-colors ${
              active
                ? 'bg-accent text-background border-accent font-semibold'
                : 'bg-surface-2 text-text-secondary border-border hover:bg-surface-3 hover:text-text-primary'
            }`}
          >
            {tab.label}
            {tab.badge != null && (
              <span className={`ml-1 text-xs ${active ? 'text-background/60' : 'opacity-75'}`}>
                ({tab.badge})
              </span>
            )}
          </button>
        );
      })}
    </div>
  );
}
