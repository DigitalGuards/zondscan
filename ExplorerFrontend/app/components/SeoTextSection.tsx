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
  itemClassName = 'card p-8 hover:border-accent/50 transition-all duration-300',
}: SeoTextSectionProps): JSX.Element {
  return (
    <section className={containerClassName}>
      {items.map((item, index) => (
        <div key={index} className={itemClassName}>
          <h2 className="text-2xl sm:text-3xl font-bold text-accent mb-4">
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
