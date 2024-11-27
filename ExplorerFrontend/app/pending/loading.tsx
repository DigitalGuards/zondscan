export default function Loading() {
  return (
    <div className="p-8">
      <h1 className="text-2xl font-bold mb-6 text-[#ffa729]">Mempool</h1>
      <p className="text-gray-400 mb-6">
        Showing unconfirmed transactions waiting to be included in a block. Updates every 5 seconds.
      </p>
      <div className="space-y-4">
        {[...Array(5)].map((_, i) => (
          <div 
            key={i}
            className="rounded-xl bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] border border-[#3d3d3d] p-6 animate-pulse"
          >
            <div className="flex flex-col md:flex-row items-center">
              <div className="w-48 flex flex-col items-center">
                <div className="w-8 h-8 bg-gray-700 rounded-lg mb-2"></div>
                <div className="h-4 w-20 bg-gray-700 rounded"></div>
              </div>
              <div className="flex-1 md:ml-8 space-y-2">
                <div className="h-6 w-32 bg-gray-700 rounded"></div>
                <div className="h-4 w-full bg-gray-700 rounded"></div>
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
