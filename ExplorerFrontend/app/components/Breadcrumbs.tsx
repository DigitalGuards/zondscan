import Link from 'next/link'
import { ChevronRightIcon } from '@heroicons/react/20/solid'

export interface BreadcrumbItem {
  label: string
  href?: string
}

interface BreadcrumbsProps {
  items: BreadcrumbItem[]
}

export default function Breadcrumbs({ items }: BreadcrumbsProps): JSX.Element {
  return (
    <nav aria-label="Breadcrumb" className="mb-4 md:mb-6">
      <ol className="flex items-center flex-wrap gap-1 text-sm text-gray-400">
        <li>
          <Link href="/" className="hover:text-[#ffa729] transition-colors">
            Home
          </Link>
        </li>
        {items.map((item, index) => {
          const isLast = index === items.length - 1
          return (
            <li key={item.label} className="flex items-center gap-1">
              <ChevronRightIcon className="w-4 h-4 text-gray-600 flex-shrink-0" />
              {isLast || !item.href ? (
                <span className="text-gray-300 truncate max-w-[200px] sm:max-w-[300px]" aria-current="page">
                  {item.label}
                </span>
              ) : (
                <Link href={item.href} className="hover:text-[#ffa729] transition-colors">
                  {item.label}
                </Link>
              )}
            </li>
          )
        })}
      </ol>
    </nav>
  )
}
