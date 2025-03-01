import React from 'react';
import { MessageSquare, Database, Zap, Shield, Server, Boxes, Rocket } from 'lucide-react';
import FloatingBackground from './FloatingBackground';

const CompactFeaturesSection: React.FC = () => {
  const features = [
    {
      icon: <MessageSquare className="w-8 h-8" />,
      title: "AI-Powered Conversations",
      description: "Ask questions, get answers, and manage your database with natural language.",
      width: "md:col-span-2 lg:col-span-2",
      transform: "rotate(-1deg)",
      importance: "primary"
    },
    {
      icon: <Database className="w-6 h-6" />,
      title: "Multi-Database Support",
      description: "Connect to PostgreSQL, YugabyteDB, MySQL, MongoDB, Redis, Neo4j and more.",
      width: "md:col-span-1 lg:col-span-1",
      transform: "rotate(1deg)",
      importance: "secondary"
    },
    {
      icon: <Zap className="w-7 h-7" />,
      title: "Query Optimization & Suggestions",
      description: "Get AI-driven suggestions to improve database performance.",
      width: "md:col-span-1 lg:col-span-1",
      transform: "rotate(-0.5deg)",
      importance: "secondary"
    },
    {
      icon: <Shield className="w-8 h-8" />,
      title: "Self-Hosted & Open Source",
      description: "NeoBase gives you the ultimate control. Choose the LLM client of your choice & deploy on your own infrastructure. No data leaves your infrastructure unless you want it to.",
      width: "md:col-span-2 lg:col-span-2",
      transform: "rotate(0.7deg)",
      importance: "secondary"
    },
    {
      icon: <Server className="w-6 h-6" />,
      title: "Query Execution & Transaction Management",
      description: "Execute queries, rollback if needed, and visualize large volumes of data with ease.",
      width: "md:col-span-2 lg:col-span-2",
      transform: "rotate(-0.8deg)",
      importance: "primary"
    },
    {
      icon: <Boxes className="w-6 h-6" />,
      title: "Smart Schema Management",
      description: "NeoBase manages your database schema for you, while giving you the flexibility to control it.",
      width: "md:col-span-1 lg:col-span-1",
      transform: "rotate(0.5deg)",
      importance: "secondary"
    }
  ];

  return (
    <section id="features" className="py-20 md:py-24 bg-[#FFDB58]/5 relative overflow-hidden">
      <FloatingBackground count={10} opacity={0.03} />
      
      <div className="container mx-auto px-6 md:px-8 max-w-7xl">
        <div className="text-center mb-12">
          <h2 className="text-3xl md:text-4xl font-bold mb-4">
            <span className="text-yellow-500">Features</span> Your Data deserves
          </h2>
          <p className="text-lg text-gray-700 max-w-3xl mx-auto">
            NeoBase makes database management simple and intuitive with these powerful features.
          </p>
        </div>
        
        <div className="grid grid-cols-1 md:grid-cols-3 lg:grid-cols-3 gap-6 md:gap-8 lg:gap-10">
          {features.map((feature, index) => (
            <div 
              key={index} 
              className={`neo-border bg-white p-5 hover:shadow-lg transition-all duration-300 ${feature.width}`}
              style={{ 
                transform: feature.transform,
                zIndex: feature.importance === "primary" ? 10 : feature.importance === "secondary" ? 5 : 1
              }}
            >
              <div className={`flex ${feature.importance === "primary" ? "flex-col items-start" : "items-start"}`}>
                <div className={`bg-[#FFDB58]/20 p-3 rounded-lg ${feature.importance === "primary" ? "mb-4" : "mr-4"} ${index % 2 === 0 ? 'self-start' : 'self-center'}`}>
                  {feature.icon}
                </div>
                <div>
                  <h3 className={`font-bold mb-2 ${feature.importance === "primary" ? "text-xl" : feature.importance === "secondary" ? "text-lg" : "text-base"}`}>
                    {feature.title}
                  </h3>
                  <p className={`text-gray-600 ${feature.importance === "primary" ? "text-base" : "text-sm"}`}>
                    {feature.description}
                  </p>
                </div>
              </div>
            </div>
          ))}
        </div>
        
        <div className="mt-16 text-center">
          <a 
            href="https://github.com/bhaskarblur/neobase-ai-dba" 
            target="_blank" 
            rel="noopener noreferrer" 
            className="neo-button inline-flex items-center justify-center gap-2 py-3 px-8 text-lg"
          >
            <span className="flex items-center gap-4"><Rocket className="w-5 h-5" /> Try NeoBase</span>
          </a>
        </div>
      </div>
    </section>
  );
};

export default CompactFeaturesSection; 