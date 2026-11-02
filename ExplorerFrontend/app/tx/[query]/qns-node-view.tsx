import Link from 'next/link';
import AddressText from '@/app/components/AddressText';
import DetailRow from '@/app/components/DetailRow';
import type { QnsNodeRecord } from './hash-fallback';

/**
 * Shown on /tx/<hash> when the hash is a registered QNS name hash. The name
 * appears only when the resolver publishes one that hashes back to the node.
 */
export default function QnsNodeView({ record }: { record: QnsNodeRecord }): JSX.Element {
  return (
    <main className="detail-content" aria-labelledby="qns-node-heading">
      <section aria-labelledby="qns-node-heading" className="card overflow-hidden mb-6">
        <div className="p-4 sm:p-6 border-b border-border">
          <h1 id="qns-node-heading" className="section-title">This is a QNS name hash</h1>
          <p className="text-sm text-text-secondary mt-2 max-w-prose">
            This hash identifies a name in the QNS registry. It matches no transaction or block.
          </p>
        </div>
        <div className="p-4 sm:p-6">
          <DetailRow label="Name hash" mono>{record.node}</DetailRow>
          <DetailRow label="Name">
            {record.name ? (
              <Link
                href={`/address/${record.name}`}
                className="text-accent hover:text-accent-hover transition-colors"
              >
                {record.name}
              </Link>
            ) : (
              <span className="text-text-secondary">Name not published</span>
            )}
          </DetailRow>
          <DetailRow label="Owner" mono>
            <Link
              href={`/address/${record.owner}`}
              className="text-accent hover:text-accent-hover transition-colors"
            >
              <AddressText address={record.owner} />
            </Link>
          </DetailRow>
          <DetailRow label="Resolver" mono>
            {record.resolver ? (
              <Link
                href={`/address/${record.resolver}`}
                className="text-accent hover:text-accent-hover transition-colors"
              >
                <AddressText address={record.resolver} />
              </Link>
            ) : (
              <span className="text-text-secondary">No resolver set</span>
            )}
          </DetailRow>
        </div>
      </section>
    </main>
  );
}
