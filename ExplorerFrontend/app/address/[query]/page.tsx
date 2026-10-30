import type { Metadata } from 'next';
import { cache } from 'react';
import AddressView from './address-view';
import TokenContractView from './token-contract-view';
import { loadAddressPageData, reverseQnsAddress } from './address-page-data';
import { AddressPageFailureView, generateAddressPageMetadata } from './address-page-presentation';
import { canonicalizeQrlAddress } from '../../lib/qrlAddress';
import { normalizeQnsName } from '../../lib/qns';
import config from '../../../config';

interface PageProps {
  params: Promise<{ query: string }>;
  searchParams?: Promise<Record<string, string | string[]>>;
}

const loadPageData = cache((query: string) => loadAddressPageData(query, config.handlerUrl));

export async function generateMetadata({
  params,
}: {
  params: Promise<{ query: string }>;
}): Promise<Metadata> {
  const resolvedParams = await params;
  const address = canonicalizeQrlAddress(resolvedParams.query);
  const name = address ? await reverseQnsAddress(address, config.handlerUrl) : undefined;
  return generateAddressPageMetadata(resolvedParams.query, name);
}

export default async function Page({ params }: PageProps): Promise<JSX.Element> {
  const resolvedParams = await params;
  const result = await loadPageData(resolvedParams.query);
  if (!result.ok) return <AddressPageFailureView failure={result} />;

  const { address, addressData, qnsName } = result;
  const handlerUrl = config.handlerUrl ?? '';
  const isTokenContract = addressData.contract_code?.isToken === true;

  return (
    <main>
      <h1 className="sr-only">
        {qnsName
          ? normalizeQnsName(resolvedParams.query)
            ? `${qnsName} resolves to ${address}`
            : `Address ${address} with the verified QNS primary name ${qnsName}`
          : `Address ${address}`}
      </h1>
      {isTokenContract && addressData.contract_code ? (
        <TokenContractView
          address={address}
          contractData={addressData.contract_code}
          handlerUrl={handlerUrl}
          qnsName={qnsName}
        />
      ) : (
        <AddressView addressData={addressData} addressSegment={address} qnsName={qnsName} />
      )}
    </main>
  );
}
