import React from 'react';
import FloatingBackground from './FloatingBackground';
import { Check, Clock, Database as DatabaseIcon, BrainCircuit } from 'lucide-react';

interface Technology {
  name: string;
  isSupported: boolean;
  priority?: number;
}

const SupportedTechnologiesSection: React.FC = () => {
  const databases: Technology[] = [
    { name: 'PostgreSQL', isSupported: true },
    { name: 'YugabyteDB', isSupported: true },
    { name: 'MongoDB', isSupported: true, priority: 1 },
    { name: 'MySQL', isSupported: false, priority: 2 },
    { name: 'Neo4j', isSupported: false, priority: 3 },
    { name: 'Redis', isSupported: false, priority: 4 },
    { name: 'Clickhouse', isSupported: true, priority: 5 }
  ];

  const llmClients: Technology[] = [
    { name: 'OpenAI', isSupported: true },
    { name: 'Google Gemini', isSupported: true },
    { name: 'Anthropic (Claude)', isSupported: false, priority: 1 },
    { name: 'Ollama', isSupported: false, priority: 2 }
  ];

  const TechnologyChip = ({ tech }: { tech: Technology }) => (
    <div className="neo-border bg-white px-4 py-3 flex items-center gap-3 transition-all hover:shadow-lg">
      <div className={`${tech.isSupported ? 'bg-green-100 text-green-600' : 'bg-red-100 text-red-500'} p-1.5 rounded-full`}>
        {tech.isSupported ? <Check size={16} /> : <Clock size={16} />}
      </div>
      <span className="font-bold">{tech.name}</span>
    </div>
  );

  return (
    <section id="technologies" className="mt-20 py-20 md:py-28 bg-white relative overflow-hidden">
      <FloatingBackground count={18} opacity={0.03} />
      
      {/* Additional floating logos with different sizes */}
      <div className="absolute inset-0 -z-10 overflow-hidden">
        {Array.from({ length: 5 }).map((_, i) => (
          <img 
            key={`db-${i}`}
            src="/db-logos/postgresql.svg" 
            alt="" 
            className="absolute w-12 h-12 animate-float opacity-10"
            style={{
              top: `${Math.random() * 100}%`,
              left: `${Math.random() * 30}%`,
              animationDelay: `${Math.random() * 10}s`,
              animationDuration: `${Math.random() * 10 + 20}s`,
              transform: `rotate(${Math.random() * 360}deg)`,
            }}
          />
        ))}
        
        {Array.from({ length: 3 }).map((_, i) => (
          <img 
            key={`mongo-${i}`}
            src="/db-logos/mongodb.svg" 
            alt="" 
            className="absolute w-14 h-14 animate-float opacity-10"
            style={{
              top: `${Math.random() * 100}%`,
              left: `${30 + Math.random() * 40}%`,
              animationDelay: `${Math.random() * 10}s`,
              animationDuration: `${Math.random() * 10 + 20}s`,
              transform: `rotate(${Math.random() * 360}deg)`,
            }}
          />
        ))}
        
        {Array.from({ length: 4 }).map((_, i) => (
          <img 
            key={`mysql-${i}`}
            src="/db-logos/mysql.svg" 
            alt="" 
            className="absolute w-10 h-10 animate-float opacity-10"
            style={{
              top: `${Math.random() * 100}%`,
              left: `${70 + Math.random() * 30}%`,
              animationDelay: `${Math.random() * 10}s`,
              animationDuration: `${Math.random() * 10 + 20}s`,
              transform: `rotate(${Math.random() * 360}deg)`,
            }}
          />
        ))}
      </div>
      
      <div className="container mx-auto px-6 md:px-8 max-w-7xl">
        <div className="text-center mb-16">
          <h2 className="text-3xl md:text-4xl font-bold mb-4">
            Supported <span className="text-green-500">Technologies</span>
          </h2>
          <p className="text-lg text-gray-700 max-w-3xl mx-auto">
            NeoBase works with a variety of databases and LLM clients, with more being added regularly.
          </p>
        </div>
        
        <div className="flex flex-col lg:flex-row gap-12 lg:gap-16">
          {/* Databases Section */}
          <div className="flex-1">
            <div className="neo-border bg-[#FFDB58]/5 p-6 h-full">
              <h3 className="text-2xl font-bold mb-6 flex items-center">
                <DatabaseIcon className="mr-2 w-6 h-6" /> Databases
              </h3>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                {databases.map((db, index) => (
                  <TechnologyChip key={index} tech={db} />
                ))}
              </div>
            </div>
          </div>

          {/* LLM Clients Section */}
          <div className="flex-1">
            <div className="neo-border bg-[#FFDB58]/5 p-6 h-full">
              <h3 className="text-2xl font-bold mb-6 flex items-center">
                <BrainCircuit className="mr-2 w-6 h-6" /> LLM Clients
              </h3>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                {llmClients.map((llm, index) => (
                  <TechnologyChip key={index} tech={llm} />
                ))}
              </div>
            </div>
          </div>
        </div>

        <div className="mt-8 text-center">
          <p className="text-gray-600 italic">
            Don't see your database or LLM client? <a href="https://github.com/bhaskarblur/neobase-ai-dba/issues" className="text-green-600 hover:text-green-700 underline font-medium">Raise a request</a>
          </p>
        </div>
      </div>
    </section>
  );
};

export default SupportedTechnologiesSection; 