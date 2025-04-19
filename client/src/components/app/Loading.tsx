import { Sparkles, Loader2 } from "lucide-react";
import { useState, useEffect } from "react";

  // Use case examples for the loading screen
  const useCaseExamples = [
    "Analyze sales performance with 'Show me top-selling products this quarter'",
    "Create executive reports with 'Generate a summary of monthly revenue by region'",
    "Identify trends with 'How has customer acquisition changed over the past year?'",
    "Monitor KPIs by asking 'What's our current customer retention rate?'",
    "Track inventory with 'Show me products with less than 10 units in stock'",
    "Analyze HR data with 'What's our employee turnover rate by department?'",
    "Get financial insights with 'Compare Q1 expenses to our annual budget'"
  ];

  // Loading component with enhanced visual appeal
  const LoadingComponent = () => {
    const [useCaseIndex, setUseCaseIndex] = useState(Math.floor(Math.random() * useCaseExamples.length));
    
    useEffect(() => {
      // Rotate through use cases every 5 seconds
      const interval = setInterval(() => {
        setUseCaseIndex(prev => (prev + 1) % useCaseExamples.length);
      }, 5000);
      
      return () => clearInterval(interval);
    }, []);
    
    return (
      <div className="h-screen max-h-screen overflow-hidden flex flex-col items-center justify-center bg-gradient-to-b from-white to-gray-50 p-4">
        <div className="flex flex-col items-center justify-center max-w-md w-full text-center">
          {/* Simple circular loader with yellow theme */}
          <div className="w-16 h-16 mb-6 flex items-center justify-center">
            <Loader2 className="w-16 h-16 text-[#FFDB58] animate-spin" />
          </div>
          
          <h2 className="text-2xl font-bold mb-2 flex items-center">
            Loading NeoBase <Sparkles className="w-5 h-5 ml-2 text-[#FFDB58]" />
          </h2>
          
          <p className="text-gray-600 mb-8">Your AI-powered database copilot is preparing...</p>
          
          {/* Random use case card */}
          <div className="neo-border bg-white p-4 rounded-lg mb-6 transition-all duration-500 ease-in-out">
            <div className="flex items-start">
              <div className="bg-[#FFDB58]/20 p-2 rounded-lg mr-3">
                <Sparkles className="w-4 h-4 text-gray-800" />
              </div>
              <div className="text-left">
                <h3 className="font-medium text-sm mb-1">What can Neobase do?</h3>
                <p className="text-sm text-gray-600">{useCaseExamples[useCaseIndex]}</p>
              </div>
            </div>
          </div>
          
        </div>
      </div>
    );
  };

  export default LoadingComponent;