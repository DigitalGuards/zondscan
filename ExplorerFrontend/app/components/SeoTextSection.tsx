import React from 'react';

export interface SeoTextItem {
  title: string;
  text: string;
}

interface SeoTextSectionProps {
  items: SeoTextItem[];
  containerClassName?: string;
  itemClassName?: string;
}

export default function SeoTextSection({
  items,
  containerClassName = 'w-full grid grid-cols-1 md:grid-cols-2 gap-8 mt-16 mb-12',
  itemClassName = 'rounded-2xl bg-gradient-to-br from-[#2d2d2d]/90 to-[#1f1f1f]/90 border border-[#3d3d3d] shadow-xl p-8 hover:border-[#ffa729]/50 transition-all duration-300',
}: SeoTextSectionProps) {
  return (
    <section className={containerClassName}>
      {items.map((item, index) => (
        <div key={index} className={itemClassName}>
          <h2 className="text-2xl sm:text-3xl font-bold text-[#ffa729] mb-4">
            {item.title}
          </h2>
          <p className="text-base sm:text-lg leading-relaxed text-gray-300/90">
            {item.text}
          </p>
        </div>
      ))}
    </section>
  );
}
