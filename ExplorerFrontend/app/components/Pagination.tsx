import Link from "next/link";

interface PaginationProps {
  postsPerPage: number;
  totalPosts: number;
  paginate: (pageNumber: number) => void;
}

export default function Pagination({
  postsPerPage,
  totalPosts,
  paginate
}: PaginationProps): JSX.Element {
  const pageNumbers: number[] = [];

  for (let i = 1; i <= Math.ceil(totalPosts / postsPerPage); i++) {
    pageNumbers.push(i);
  }

  const handleClick = (number: number): void => {
    paginate(number);
  };

  return (
    <nav aria-label="Pagination">
      <ul className="flex flex-wrap items-center gap-1">
        {pageNumbers.map((number) => (
          <li key={number}>
            <Link
              onClick={() => handleClick(number)}
              href={`#${number}`}
              className="inline-flex items-center justify-center min-w-[36px] h-9 px-3
                         text-sm font-medium text-gray-300
                         bg-background-secondary border border-border rounded
                         hover:bg-border hover:text-white transition-colors"
            >
              {number}
            </Link>
          </li>
        ))}
      </ul>
    </nav>
  );
}
