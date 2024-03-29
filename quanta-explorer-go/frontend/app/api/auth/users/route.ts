import { NextResponse } from "next/server";
import startDb from "../../../lib/db";
import UserModel from "../../../models/userModels";

interface NewUserRequest {
    ethAddress: string;
    role: string;
}

interface NewUserResponse {
    ethAddress: string;
    role: string;
}

type NewResponse = NextResponse<{ user?: NewUserResponse; error?: string}>;

export const POST  = async (req: Request): Promise<NewResponse> => {
    const body = (await req.json()) as NewUserRequest;

    await startDb();

    const oldUser = await  UserModel.findOne({ id: body.ethAddress});
    
    if (oldUser)
        return NextResponse.json(
            {error: "ethAddress is already in use!"},
            {status: 422},
        );

    const user = await UserModel.create({ ...body});

    return NextResponse.json({
        user: {
            ethAddress: user.ethAddress,
            role: user.role,
        }
    })
};