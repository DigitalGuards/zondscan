'use client';

import { epochToISO } from '../../lib/helpers';
import type { ActivityDisplayProps } from './types';

export default function ActivityDisplay({ firstSeen, lastSeen }: ActivityDisplayProps): JSX.Element {
  return (
    <div className="relative overflow-hidden rounded-xl 
                  bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f]
                  border border-[#3d3d3d] p-4 md:p-6">
      <h2 className="text-sm font-semibold text-gray-400 mb-3 md:mb-4">Activity</h2>
      {(epochToISO(firstSeen) === "1970-01-01" && epochToISO(lastSeen) === "1970-01-01") ? (
        <p className="text-sm md:text-base text-gray-300">No transactions were signed from this wallet yet.</p>
      ) : (
        <div className="grid grid-cols-2 md:space-y-2 gap-3 md:gap-0">
          <div>
            <div className="text-xs md:text-sm text-gray-400">First Activity</div>
            <div className="text-sm md:text-base text-gray-300">{epochToISO(firstSeen).split('T')[0]}</div>
          </div>
          <div>
            <div className="text-xs md:text-sm text-gray-400">Last Activity</div>
            <div className="text-sm md:text-base text-gray-300">{epochToISO(lastSeen).split('T')[0]}</div>
          </div>
        </div>
      )}
    </div>
  );
}
