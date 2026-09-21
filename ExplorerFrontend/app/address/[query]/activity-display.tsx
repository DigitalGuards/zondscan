'use client';
import InterfaceText from '../../components/InterfaceText';

import TimeDisplay from '../../components/TimeDisplay';
import type { ActivityDisplayProps } from '@/app/types';

export default function ActivityDisplay({ firstSeen, lastSeen }: ActivityDisplayProps): JSX.Element {
  return (
    <div className="relative overflow-hidden rounded-lg md:rounded-xl 
                  bg-card-gradient
                  border border-border p-3 md:p-4 lg:p-6">
      <h2 className="text-xs md:text-sm font-semibold text-text-secondary mb-2 md:mb-3 lg:mb-4"><InterfaceText text="Activity" /></h2>
      {(!firstSeen && !lastSeen) ? (
        <p className="text-xs md:text-sm lg:text-base text-text-secondary">No transactions were signed from this wallet yet.</p>
      ) : (
        <div className="grid grid-cols-2 gap-2 md:gap-3 lg:gap-4">
          <div>
            <div className="text-[10px] md:text-xs lg:text-sm text-text-secondary"><InterfaceText text="First Activity" /></div>
            <div className="text-xs md:text-sm lg:text-base text-text-secondary"><TimeDisplay timestamp={firstSeen} dateOnly /></div>
          </div>
          <div>
            <div className="text-[10px] md:text-xs lg:text-sm text-text-secondary"><InterfaceText text="Last Activity" /></div>
            <div className="text-xs md:text-sm lg:text-base text-text-secondary"><TimeDisplay timestamp={lastSeen} dateOnly /></div>
          </div>
        </div>
      )}
    </div>
  );
}
