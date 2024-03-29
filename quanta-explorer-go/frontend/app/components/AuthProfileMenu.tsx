"use client";


// signOut
import { useSession } from "next-auth/react";
import Link from "next/link";
import React from "react";
import { useState } from 'react';

export default function AuthProfileMenu() {
    const [tab, setTab] = useState('profile');

    const {status} = useSession();

    console.log(status);

    const isAuth = status === "authenticated";

    if (isAuth)

    return (
        <div className="container mx-auto p-4">
          <div className="flex gap-4 mb-4">
            <button className={`tab-button ${tab === 'profile' ? 'tab-active' : ''}`} onClick={() => setTab('profile')}>Profile</button>
            <button className={`tab-button ${tab === 'FS' ? 'tab-active' : ''}`} onClick={() => setTab('FS')}>QRL Funding System (QRL-FS)</button>
            <button className={`tab-button ${tab === 'votes' ? 'tab-active' : ''}`} onClick={() => setTab('votes')}>Votes</button>
            <button className={`tab-button ${tab === 'analytics' ? 'tab-active' : ''}`} onClick={() => setTab('analytics')}>Premium Analytics</button>
            <button className={`tab-button ${tab === 'settings' ? 'tab-active' : ''}`} onClick={() => setTab('settings')}>Settings</button>
            <button className={`tab-button ${tab === 'ems' ? 'tab-active' : ''}`} onClick={() => setTab('ems')}>EMS</button>
          </div>
    
          <div className="tab-content">
            {tab === 'profile' && <p>Public Profile Page Content - Public profile, Comment history under community proposals, transaction activity, QRL balance</p>}
            {tab === 'FS' && <p>Community funding proposals</p>}
            {tab === 'votes' && <p>Create Public Votes Content - ability to vote and see votes</p>}
            {tab === 'analytics' && <p>Premium Analytics Content - analytics here</p>}
            {tab === 'settings' && <p>Settings Page Content - Set usernames here and preferences</p>}
            {tab === 'ems' && <p>Ephemeral Messenger System Content - ability to chat with another wallet</p>}
          </div>
        </div>
      )

    return (
        <ul className="flex items-center space-x-6">
            <Link href="/auth/sign-in">Login (currently Guest)</Link>
        </ul>
    )

}

