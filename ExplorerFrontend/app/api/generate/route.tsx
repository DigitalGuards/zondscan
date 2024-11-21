/* eslint-disable @typescript-eslint/no-unused-vars */
/* eslint-disable @typescript-eslint/no-explicit-any */

// FIX LATER ESLINT
import { NextResponse} from "next/server";
import { PrismaClient } from '@prisma/client'
import crypto from 'crypto';

const prisma = new PrismaClient()

// Make this a POST request and store challenge and associated fields:
// blockchain_address, challenge, timestamp, and status in MongoDB
export async function POST (req: any, res: any){
    // const body = await req.json()
    // const randomMessage = crypto.randomBytes(64).toString('hex');

    // const challenge = await prisma.challenge.create({
    //     data: {
    //       blockchain_address: body.address,
    //       challenge: "jsdkfsnkfnsdkfjsbnfj",
    //       status: 0,
    //     },
    //   })
      
    return NextResponse.json("jsdkfsnkfnsdkfjsbnfj");
}