"use client"

import Link from 'next/link';

export default () => {

    const footerNavs = [
        {
            label: "Resources",
            items: [
                {
                    href: 'https://www.theqrl.org/discord',
                    name: 'Feedback & Support (Discord)'
                },
                {
                    href: 'https://q-day.org/',
                    name: 'Visit Q-Day to learn more about the quantum threat'
                },
                // {
                //     href: 'javascript:void()',
                //     name: 'About Quanta Explorer'
                // },
            ],
        },
        // {
        //     label: "Legal",
        //     items: [
        //         {
        //             href: 'javascript:void()',
        //             name: 'Privacy'
        //         },
        //         {
        //             href: 'javascript:void()',
        //             name: 'Terms of Services'
        //         },
        //     ]
        // },
        {
            label: "Learn more",
            items: [
                {
                    href: 'https://www.theqrl.org/',
                    name: 'Visit the QRL website'
                },
                {
                    href: 'https://wallet.theqrl.org/',
                    name: 'Use the QRL Web Wallet'
                },
            ]
        },
        // {
        //     label: "Company",
        //     items: [
        //         {
        //             href: 'javascript:void()',
        //             name: 'Partners'
        //         },
        //         {
        //             href: 'javascript:void()',
        //             name: 'Team'
        //         },
        //     ],
        // }
    ]

    return (
        <footer className="pt-10">
            <div className="max-w-screen-xl mx-auto px-4 md:px-8">
                <div className="flex-1 mt-16 space-y-6 justify-between sm:flex md:space-y-0">
                    {
                        footerNavs.map((item, idx) => (
                            <ul
                                className="space-y-4 text-gray-600"
                                key={idx}
                            >
                                <h4 className="text-gray-800 font-semibold sm:pb-2">
                                    {item.label}
                                </h4>
                                {
                                    item.items.map(((el, idx) => (
                                        <li key={idx}>
                                            <Link
                                                href={el.href}
                                                className="hover:text-gray-800 duration-150"

                                            >
                                                {el.name}
                                            </Link>
                                        </li>
                                    )))
                                }
                            </ul>
                        ))
                    }
                </div>
                <div className="mt-10 py-10 border-t items-center justify-between sm:flex">
                    <p className="text-gray-600">Quanta Explorer</p>
                </div>
            </div>
        </footer>
    )
}