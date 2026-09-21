import type { Metadata } from 'next';
import AddressView from './address-view';
import TokenContractView from './token-contract-view';
import { loadAddressPageData } from './address-page-data';
import { AddressPageFailureView, generateAddressPageMetadata } from './address-page-presentation';
import config from '../../../config';

interface PageProps {
  params: Promise<{ query: string }>;
  searchParams?: Promise<Record<string, string | string[]>>;
}

export async function generateMetadata({
  params,
}: {
  params: Promise<{ query: string }>;
}): Promise<Metadata> {
  const resolvedParams = await params;
  return generateAddressPageMetadata(resolvedParams.query);
}

export default async function Page({ params }: PageProps): Promise<JSX.Element> {
  const resolvedParams = await params;
  const result = await loadAddressPageData(resolvedParams.query, config.handlerUrl);
  if (!result.ok) return <AddressPageFailureView failure={result} />;

  const { address, addressData, qnsName } = result;
  const handlerUrl = config.handlerUrl ?? '';
  const isTokenContract = addressData.contract_code?.isToken === true;

  return (
    <main>
      <h1 className="sr-only">
        {qnsName ? `${qnsName} resolves to ${address}` : `Address ${address}`}
      </h1>
      {isTokenContract ? (
        <TokenContractView
          address={address}
          contractData={addressData.contract_code!}
          handlerUrl={handlerUrl}
          qnsName={qnsName}
        />
      ) : (
        <AddressView
          addressData={addressData}
          addressSegment={address}
          qnsName={qnsName}
        />
      )}
    </main>
  );
}
