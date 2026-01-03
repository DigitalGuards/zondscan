import Link from 'next/link'

export default function NotFound(): JSX.Element {
  return (
    <div className="min-h-screen flex items-center justify-center">
      <div className="text-center">
        <h2 className="text-4xl font-bold text-accent mb-4">404 Not Found</h2>
        <p className="text-gray-300 mb-8">Could not find the requested resource</p>
        <Link
          href="/"
          className="btn-primary px-6 py-3"
        >
          Return Home
        </Link>
      </div>
    </div>
  )
}
