'use client';

export interface TabPill {
  key: string;
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
 */
export default function TabPillBar({
  tabs,
  activeKey,
  onSelect,
  ariaLabel,
  withPanelIds = false,
}: {
  tabs: TabPill[];
  activeKey: string;
  onSelect: (key: string) => void;
  ariaLabel: string;
  withPanelIds?: boolean;
}): JSX.Element {
  return (
    <div
      role="tablist"
      aria-label={ariaLabel}
      className="flex gap-2 overflow-x-auto scrollbar-none -mx-1 px-1 py-1"
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
            className={`whitespace-nowrap shrink-0 rounded-lg px-3 md:px-4 py-1.5 md:py-2 text-xs md:text-sm font-medium transition-colors ${
              active
                ? 'bg-[#ffa729] text-black'
                : 'bg-[#2d2d2d] text-gray-300 hover:bg-[#3d3d3d] hover:text-white'
            }`}
          >
            {tab.label}
            {tab.badge != null && (
              <span className={`ml-1 text-xs ${active ? 'text-black/60' : 'opacity-75'}`}>
                ({tab.badge})
              </span>
            )}
          </button>
        );
      })}
    </div>
  );
}
