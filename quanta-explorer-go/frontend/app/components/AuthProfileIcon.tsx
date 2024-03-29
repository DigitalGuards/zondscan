"use client";

import { useSession, signOut } from "next-auth/react";
import Image from 'next/image';
import UserCircleIcon from '../../public/user-circle.svg'
import Link from "next/link";
import React from "react";

export default function AuthProfileMenu() {
    const { data, status } = useSession();

    console.log(status);

    const isAuth = status === "authenticated";

    if (isAuth)

        return (
            <>
                <div className="hidden lg:flex lg:flex-1 lg:justify-end"></div>
                <div className="mr-2">
                    Welcome, {data?.user?.name}! |
                </div>
                <Link href={'/profile'}>
                    <Image
                        priority
                        src={UserCircleIcon}
                        alt="Profile Icon"
                    />
                </Link>
                <p className="ml-2 mr-2"> | </p>
                <Link href="/" onClick={() => signOut()} className="text-sm font-semibold leading-6 text-gray-900" passHref>
                    Log Out 🤖
                </Link>
                <div />
            </>
        );

    return (
        <>
            <div className="hidden lg:flex lg:flex-1 lg:justify-end"></div>
            {/* <Link href="/auth/sign-in" className="text-sm font-semibold leading-6 text-gray-900">
                Log in 🤖
            </Link> */}
            <div />
        </>
    )

}

