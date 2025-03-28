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
  containerClassName = 'w-full flex flex-wrap justify-between gap-6',
  itemClassName = 'w-full lg:w-[48%] flex flex-col py-6 px-4',
}: SeoTextSectionProps) {
  return (
    <section className={containerClassName}>
      {items.map((item, index) => (
        <div key={index} className={itemClassName}>
          <h2 className="text-2xl sm:text-3xl font-bold text-[#ffa729] mb-2">
            {item.title}
          </h2>
          <p className="text-base sm:text-lg leading-relaxed text-gray-300">
            {item.text}
          </p>
        </div>
      ))}
    </section>
  );
}
