import React, { ReactNode } from 'react';
import { getServerSession } from 'next-auth';
import { authOptions } from '../api/auth/[...nextauth]/route';
import { redirect } from "next/navigation";

interface Props {
    children: ReactNode;
}

export default async function AdminLayout({ children }: Props){
    const session = await getServerSession(authOptions);

    const user = session?.user as { role: string } | undefined;
    const isAdmin = user?.role === "admin";

    if (!isAdmin) redirect("/auth/sign-in");

    return <>{children}</>;
}