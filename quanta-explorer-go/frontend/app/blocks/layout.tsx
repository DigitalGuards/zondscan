export default function BlocksLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <main className="flex-1 overflow-y-auto bg-[#1a1a1a] pt-24 pb-12 px-2 ml-[60px]">
      {children}
    </main>
  )
}
