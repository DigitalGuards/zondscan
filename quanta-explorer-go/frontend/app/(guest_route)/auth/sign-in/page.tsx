/* eslint-disable prefer-const */
/* eslint-disable @typescript-eslint/no-unused-vars */
/* eslint-disable @typescript-eslint/no-explicit-any */
/* eslint-disable @typescript-eslint/no-var-requires */
"use client";
// FIX THIS LATER ESLINT... WORK IN PROGRESS
import React, { useState, useEffect } from 'react';
import { signIn } from 'next-auth/react';
import { useRouter } from "next/navigation";
import Alert from "../../../components/Alert";
// import { Kyber1024 } from "crystals-kyber-js";
import * as dilithium5 from '@theqrl/dilithium5';
const { Web3 } = require('web3');
const Buffer = require('buffer/').Buffer


function testWrapper(ref: any, test: any, description: any) {
  try {
    console.log(ref);
    console.log(test);
    console.log(description);
    if (ref === test) {
      console.log(`✅ ${description} pass`);
    } else {
      console.log(`❌ ${description} fail`);
    }
    return;
  } catch (e) {
    console.log(`❌ ${description} fail`);
  }
}

function HexSeedToSKs(hexSeed: string) {
  // Convert hex seed back to byte array
  const byteArray = new Uint8Array(hexSeed.length / 2);
  for (let i = 0, j = 0; i < hexSeed.length; i += 2, j++) {
      byteArray[j] = parseInt(hexSeed.substr(i, 2), 16);
  }

  const delimiter = new Uint8Array([255, 255, 255, 255]);

  // Find the delimiter and split into original keys
  let delimiterIndex = -1;
  for (let i = 0; i < byteArray.length - delimiter.length + 1; i++) {
      let match = true;
      for (let j = 0; j < delimiter.length; j++) {
          if (byteArray[i + j] !== delimiter[j]) {
              match = false;
              break;
          }
      }
      if (match) {
          delimiterIndex = i;
          break;
      }
  }

  if (delimiterIndex === -1) {
      console.error("Delimiter not found");
      return;
  }

  const recoveredDilithiumSK = byteArray.subarray(0, delimiterIndex);
  // const recoveredKyberSK = byteArray.subarray(delimiterIndex + delimiter.length);

  console.log("Recovered Dilithium SK:", recoveredDilithiumSK);

  return recoveredDilithiumSK;
}

async function generateChallenge(address: string) {
  const response = await fetch('/api/generate', {
      method: 'POST',
      headers: {
          'Content-Type': 'application/json',
      },
      body: JSON.stringify({ address: address })
  });

  if (!response.ok) {
      // Handle errors, e.g., show a message to the user
      throw new Error(`Error: ${response.statusText}`);
  }

  const challenge = "00010204060901";
  return challenge; // This will be a text string containing the challenge
}

async function getPublicKeys(privateKey: string, abi: string) {
  const web3 = new Web3(new Web3.providers.HttpProvider(``));

  let abiJSONObject = JSON.parse(JSON.stringify(abi));

  const contract = new web3.eth.Contract(abiJSONObject, "0x093901a1C69dfb0610edDFb4415c3866FC62454a");

  // Creating a signing account from a private key
  const signer = web3.eth.accounts.privateKeyToAccount(
    "0x" + privateKey
  );
  web3.eth.accounts.wallet.add(signer);

  // Assuming the contract has functions like `getDilithiumPKByAddress` and `getKyberPKByAddress`
  const dilithiumPublicKey = await contract.methods.getDilithiumPKByAddress(signer.address).call();
  const kyberPublicKey = await contract.methods.getKyberPKByAddress(signer.address).call();

  // console.log(`Dilithium Public Key: ${dilithiumPublicKey}`);
  // console.log(`Kyber Public Key: ${kyberPublicKey}`);

  return [dilithiumPublicKey, kyberPublicKey, signer.address];
}

// function byteLength(str: string) {
//   // returns the byte length of an utf8 string
//   var s = str.length;
//   for (var i = str.length - 1; i >= 0; i--) {
//     var code = str.charCodeAt(i);
//     if (code > 0x7f && code <= 0x7ff) s++;
//     else if (code > 0x7ff && code <= 0xffff) s += 2;
//     if (code >= 0xDC00 && code <= 0xDFFF) i--; //trail surrogate
//   }
//   return s;
// }

const SignIn = () => {
  const [error, setError] = useState("");
  const [dilithiumPrivateKey, setdilithiumPrivateKey] = useState("");
  const [hexSeedEphemeral, setHexSeedEphemeral] = useState("");
  const [abi, setAbi] = useState("");

  const router = useRouter();

  useEffect(() => {
    fetch('/ABI.json')
      .then(response => {
        if (!response.ok) {
          throw new Error('Network response was not ok');
        }
        return response.json();
      })
      .then(async data => {
        setAbi(data.abi);
      })
      .catch(error => {
        console.error("Error fetching ABI:", error);
      });
  }, []);

  // const handleChange = (e: unknown) => {
  //   const { name, value } = e.target;

  //   switch (name) {
  //     case 'dilithiumPrivateKey':
  //       setdilithiumPrivateKey(value);
  //       break;
  //     case 'hexSeedEphemeral':
  //       setHexSeedEphemeral(value);
  //       break;
  //   }
  // };

  const handleSubmit = async (e: any) => {
    e.preventDefault();

    const pk = new Uint8Array(dilithium5.default.CryptoPublicKeyBytes);
    let sk = new Uint8Array(dilithium5.default.CryptoSecretKeyBytes);
    dilithium5.default.cryptoSignKeypair(null, pk, sk);

    console.log(pk);
    console.log(sk);

    // const sender = new Kyber1024();
    // const [pkR, skR] = await sender.generateKeyPair();

    // let kyber_PK = "0x" + Buffer.from(pkR).toString('hex');
    
    // let delimiter = new Uint8Array([255, 255, 255, 255]);

    // let combined_keys = new Uint8Array(sk.length + delimiter.length + skR.length);
    // combined_keys.set(sk);
    // combined_keys.set(delimiter, sk.length);
    // combined_keys.set(skR, sk.length + delimiter.length);

    // let hexSeedSkKeys = Array.from(combined_keys).map(byte => byte.toString(16).padStart(2, '0')).join('');

    // console.log("Hex Seed Dilithium SK and Kyber SK:", hexSeedSkKeys);

    const [dilithiumPublicKeyyy, kyberPublicKeyyy, address] = await getPublicKeys(dilithiumPrivateKey, abi);
    // const recoveredDilithiumSK = HexSeedToSKs(hexSeedEphemeral);

    try {

      // POST request with associated fields:
      // blockchain_address, challenge, timestamp, and status.
      const userBlockchainAddress = address;
      const challenge = await generateChallenge(userBlockchainAddress);

      // console.log(skR);
      // console.log(sk);

      console.log(challenge);
      let challengeBuffer = Buffer.from(challenge, 'hex');
      let signature = Buffer.from(dilithium5.default.cryptoSign(challengeBuffer, Buffer.from(sk, 'hex'), false), 'binary').toString('hex').slice(0, -challenge.length);
      const sigMessage = dilithium5.default.cryptoSign(challengeBuffer, Buffer.from(sk, 'hex'), false);
      testWrapper(signature.toString('hex') + challenge, Buffer.from(sigMessage, 'binary').toString('hex'), '00010204060901');
      // let signature = Buffer.from(dilithium5.default.cryptoSign(challengeBuffer, Buffer.from(sk, 'hex'), false), 'binary').toString('hex').slice(0, -challenge.length);
      let signedMessage = Buffer.from(signature, 'hex');
      let dilithiumPublicKeyBuffer = Buffer.from(pk, 'hex');
  
      // // Store hexSeed Key in session storage
      // if (byteLength(kyber_PK) == 1568 && byteLength(hexSeedEphemeral) == 3168 || hexSeedEphemeral != "") {
      //   sessionStorage.setItem('hexSeedEphemeral', hexSeedEphemeral);
      //   sessionStorage.setItem('kyberPublicKey', kyber_PK);
      // } else {
      //   sessionStorage.setItem('hexSeedEphemeral', "null");
      // }

      // console.log(signature.toString('hex'));
      // console.log(challengeBuffer.toString('hex'))
      // console.log(challengeBuffer.length)

      // console.log(signedMessage.toString('hex'));
      // console.log(signedMessage.length);

      console.log(dilithiumPublicKeyBuffer.toString('hex'));
      console.log(dilithiumPublicKeyBuffer.length);

      // Dilithium Public Key can be converted to ID (convert it to SHA256)
      const res = await signIn("credentials", {
        dilithiumPublicKey: dilithiumPublicKeyBuffer.toString('hex'),
        dilithiumSignature: Buffer.from(sigMessage, 'binary').toString('hex'),
        redirect: false,
      });
  
      console.log(res);
  
      if (res?.error) return setError(res.error);
      router.replace("/dashboard");
    } catch (error) {
      console.error('Error verifying:', error);
    }
  };

  return (
    <div className="flex items-center justify-center bg-gray-100 py-4"> {/* Reduced vertical padding */}
      <form onSubmit={handleSubmit} className="bg-white p-6 rounded-lg shadow-md w-96"> {/* Adjusted padding */}
        {error && (
          <div className="mb-4">
            <Alert value={error} />
          </div>
        )}
        <div className="mb-4">
          <label htmlFor="dilithiumPrivateKey" className="block text-sm font-medium text-gray-600 mb-2">Dilithium Private Key (Zond):</label>
          <input
            type="password"
            id="dilithiumPrivateKey"
            name="dilithiumPrivateKey"
            value={dilithiumPrivateKey}
            // onChange={handleChange}
            required
            className="w-full p-2 border rounded-md"
          />
        </div>
        <div className="mb-4">
          <label htmlFor="hexSeedEphemeral" className="block text-sm font-medium text-gray-600 mb-2">Kyber Secret Key (Optional: Access to EMS):</label>
          <input
            type="password"
            id="hexSeedEphemeral"
            name="hexSeedEphemeral"
            value={hexSeedEphemeral}
            // onChange={handleChange}
            className="w-full p-2 border rounded-md"
          />
        </div>
        <button type="submit" className="w-full bg-blue-500 text-white p-2 rounded-md hover:bg-blue-600 focus:outline-none focus:border-blue-700 focus:ring focus:ring-blue-200">
          Sign In
        </button>
      </form>
    </div>
  );
};

export default SignIn;
