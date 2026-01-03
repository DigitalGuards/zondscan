export default function Loading(): JSX.Element {
  return (
    <div className="p-4 max-w-3xl mx-auto">
      <div className="bg-[#2d2d2d] shadow rounded-lg p-6">
        <div className="flex items-center mb-6">
          <div className="w-6 h-6 bg-[#ffa729]/20 rounded-full animate-pulse mr-2"></div>
          <div className="h-8 w-48 bg-[#ffa729]/20 rounded animate-pulse"></div>
        </div>
        
        <div className="space-y-4">
          {[...Array(7)].map((_, i) => (
            <div key={i} className="border-b border-gray-700 pb-4">
              <div className="h-4 w-24 bg-[#ffa729]/20 rounded mb-2 animate-pulse"></div>
              <div className="h-6 w-full bg-gray-700/20 rounded animate-pulse"></div>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
