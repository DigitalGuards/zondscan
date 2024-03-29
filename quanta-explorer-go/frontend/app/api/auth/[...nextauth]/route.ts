import NextAuth, { NextAuthOptions } from 'next-auth';
import CredentialsProvider from 'next-auth/providers/credentials';
import EthUtil from 'ethereumjs-util'
// eslint-disable-next-line @typescript-eslint/no-var-requires
const Web3 = require('@theqrl/web3');
import { cryptoSignVerify } from "@theqrl/dilithium5";

const blacklist = [""]

export const authOptions: NextAuthOptions = {
  session: {
    strategy: "jwt",
    maxAge: 48 * 60 * 60, // 48 hours in seconds
  },
  providers: [
    CredentialsProvider({
      type: "credentials",
      credentials: {
        dilithiumPublicKey: { label: "Dilithium PK", type: "text" },
        dilithiumSignature: { label: "Dilithium Signature", type: "text" },
      },
      async authorize(credentials) {
        const { dilithiumPublicKey, dilithiumSignature } = credentials as {
          dilithiumPublicKey: string,
          dilithiumSignature: string;
        }

        // TODO: The generated message by the server needs to be here
        const randomMessage = "00010204060901"; 
        const message = Buffer.from(randomMessage, 'hex');

        console.log(message.toString('hex'))
        console.log(message.length)

        const pattern = /00010204060901$/;

        const cleanedString = dilithiumSignature.replace(pattern, '');

        const dilithiumPKBuffer = Buffer.from(dilithiumPublicKey, 'hex');
        const dilithiumSignatureBuffer = Buffer.from(cleanedString, 'hex');

        console.log(cleanedString)
        console.log(dilithiumSignatureBuffer.length)

        console.log(dilithiumPKBuffer.toString('hex'))
        console.log(dilithiumPKBuffer.length)

        const isValid = cryptoSignVerify(dilithiumSignatureBuffer, message, dilithiumPKBuffer);
        console.log(isValid);
        if (!isValid) throw new Error("Invalid Dilithium Signature!");

        const signed = new Web3(dilithiumSignature)
        const dilithiumAddress = EthUtil.bufferToHex(signed.getSenderAddress());

        let userRole = "user"
        if (dilithiumAddress == process.env.ADMIN_PUBLIC_ADDRESS && isValid){
          userRole = "admin"
        }

        const blacklistSet = new Set(blacklist);
        if (blacklistSet.has(dilithiumAddress)) {
            return;
        }
        
        return {
          id: dilithiumAddress,
          role: userRole
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        } as any
      },
    }),
    ],
callbacks: {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  jwt(params: any){
    if (params.user?.role) {
      params.token.ethAddress = params.user.ethAddress;
      params.token.role = params.user.role;
    }

    return params.token;
  },
  session({ session, token }){
    if (session.user) {
      (session.user as { ethAddress: string }).ethAddress = token.ethAddress as string;
      (session.user as { role: string }).role = token.role as string;
    }
    return session;
  },
},
};

const authHandler = NextAuth(authOptions);

export { authHandler as GET, authHandler as POST };