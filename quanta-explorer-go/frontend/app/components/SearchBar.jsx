"use client"

import React, { useCallback } from 'react';
import { useRouter, usePathname } from 'next/navigation'

function onlyNumbers(str) {
  return /^[0-9]+$/.test(str);
}

export default function SearchBar() {
  const [searchValue, setSearchValue] = React.useState('');
  const router = useRouter();
  const pathname = router.asPath;

  function handleInputChange(event) {
    setSearchValue(event.target.value);
  }

  const navigateHandler = useCallback(() => {
    let newPath;
    if (onlyNumbers(searchValue)) {
      newPath = "/block/" + searchValue;
    } else if (searchValue.length == 66) {
      newPath = "/tx/" + searchValue;
    } else if (searchValue.slice(0, 2) == "0x" && searchValue.length == 42) {
      newPath = "/address/" + searchValue;
    } else {
      var div = document.getElementById('error-box');
      div.innerHTML = ""
      div.innerHTML += '<div class="p-4 mb-4 text-sm text-red-400 rounded-xl bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] border border-red-400 shadow-lg" role="alert"><span class="font-medium">Invalid input!</span></div>';
      return
    }
    router.push(newPath);
  }, [router, pathname, searchValue]);

  React.useEffect(() => {
    const listener = event => {
      if (event.code === "Enter" || event.code === "NumpadEnter") {
        event.preventDefault();
        navigateHandler();
      }
    };
    document.addEventListener("keydown", listener);
    return () => {
      document.removeEventListener("keydown", listener);
    };
  }, [navigateHandler]);

  return (
    <div className="relative w-full">
      <div className="absolute inset-0 bg-[url('/circuit-board.svg')] opacity-5 rounded-2xl"></div>
      <div className="relative bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] rounded-2xl p-6 
                    shadow-xl border border-[#3d3d3d] hover:border-[#4d4d4d] transition-colors">
        <form
          onSubmit={(e) => {
            e.preventDefault();
            navigateHandler();
          }}
          className="flex flex-col sm:flex-row gap-4">
          <input
            type="text"
            placeholder="Search by Address / Txn Hash / Block.."
            className="flex-1 py-4 px-5 text-gray-300 
                     bg-[#1a1a1a] rounded-xl
                     border border-[#3d3d3d]
                     outline-none shadow-lg
                     focus:ring-2 focus:ring-[#ffa729] focus:border-transparent
                     placeholder-gray-500 transition-all duration-300
                     hover:border-[#4d4d4d]"
            value={searchValue}
            onChange={handleInputChange}
          />
          <button
            className="px-8 py-4 bg-[#ffa729] text-white 
                     rounded-xl shadow-lg font-medium whitespace-nowrap
                     hover:bg-[#ff9709] hover:shadow-2xl hover:scale-105 
                     active:scale-95 transition-all duration-300 
                     sm:w-auto w-full"
            onClick={navigateHandler}
          >
            Search
          </button>
        </form>
        <div id="error-box" className="mt-4"></div>
      </div>
    </div>
  );
}
