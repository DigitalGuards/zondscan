"use client";

import Link from "next/link";
import React, { ChangeEventHandler, FormEventHandler, useState } from 'react';
import { useRouter } from "next/navigation"

const SignUp = () => {
  const [busy, setBusy] = useState(false);
  const [userInfo, setUserInfo] = useState({
    name: '',
    email: '',
    password: '',
  });

  const { name, email, password } = userInfo;

  const router = useRouter();

  const handleChange: ChangeEventHandler<HTMLInputElement> = ({ target }) => {
    const { name, value } = target;

    setUserInfo({ ...userInfo, [name]: value})
  }


  // Move this to username set and preferences 
  const handleSubmit: FormEventHandler<HTMLFormElement> = async (e) => {
    setBusy(true);
    e.preventDefault();
    const res = await fetch("/api/auth/users", {
        method: "POST",
        body: JSON.stringify(userInfo),
    });
    setBusy(false);
    router.replace("/auth/sign-in")
  }; 

  return (
    <div className="min-h-screen flex items-center justify-center bg-gray-100">
      <form onSubmit={handleSubmit} className="bg-white p-8 rounded-lg shadow-md w-96">
        <div className="mb-4">
          <label htmlFor="name" className="block text-sm font-medium text-gray-600 mb-2">Name:</label>
          <input 
            type="text"
            id="name"
            name="name" 
            value={name} 
            onChange={handleChange} 
            required 
            className="w-full p-2 border rounded-md"
          />
        </div>
        <div className="mb-4">
          <label htmlFor="email" className="block text-sm font-medium text-gray-600 mb-2">Email:</label>
          <input 
            type="email"
            id="email"
            name="email" 
            value={email} 
            onChange={handleChange} 
            required 
            className="w-full p-2 border rounded-md"
          />
        </div>
        <div className="mb-4">
          <label htmlFor="password" className="block text-sm font-medium text-gray-600 mb-2">Password:</label>
          <input 
            type="password"
            id="password"
            name="password" 
            value={password} 
            onChange={handleChange} 
            required 
            className="w-full p-2 border rounded-md"
          />
        </div>
        <button type="submit" disabled={busy} className="w-full bg-blue-500 text-white p-2 rounded-md hover:bg-blue-600 focus:outline-none focus:border-blue-700 focus:ring focus:ring-blue-200">
          Sign Up
        </button>
      </form>
    </div>
  );  
};

export default SignUp;
