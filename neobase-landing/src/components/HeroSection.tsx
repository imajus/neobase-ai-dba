import { ArrowRight, Github } from 'lucide-react'
import FloatingBackground from './FloatingBackground'

const HeroSection = () => {
  return (
    <section className="py-20 md:py-28 lg:py-32 relative">
      {/* Background Pattern */}
      <FloatingBackground count={20} opacity={0.05} />

      <div className="container mt-8 mx-auto px-0 relative max-w-7xl">
        <div className="flex flex-col md:flex-row items-center">
          {/* Left Side - Hero Text */}
          <div className="md:w-1/2 space-y-6 md:pr-8 z-10">
            <div className="inline-block neo-border bg-[#FFDB58] px-4 py-2 font-bold text-sm">
              #Open Source,  #AI Database Assistant
            </div>
            <h1 className="text-4xl md:text-5xl lg:text-6xl font-extrabold leading-tight">
              Where there's a Database.<br />
              <span className="text-yellow-500">There's <span className="text-green-500">NeoBase!</span></span>
            </h1>
            <p className="text-lg md:text-xl text-gray-700">
              NeoBase connects to your database & let's you talk to your data. No boring dashboards anymore, just you & your data.
            </p>
            <div className="flex flex-col sm:flex-row gap-4 pt-6">
              <a 
                href="https://github.com/bhaskarblur/neobase-ai-dba" 
                target="_blank" 
                rel="noopener noreferrer" 
                className="neo-button flex items-center justify-center gap-2 py-3 px-8 text-lg"
              >
                <span>Get Started</span>
                <ArrowRight className="w-5 h-5" />
              </a>
              <a 
                href="https://github.com/bhaskarblur/neobase-ai-dba" 
                target="_blank" 
                rel="noopener noreferrer" 
                className="neo-button-secondary flex items-center justify-center gap-2 py-3 px-6 text-lg"
              >
                <Github className="w-5 h-5" />
                <span>View on GitHub</span>
              </a>
            </div>
          </div>

          {/* Right Side - Hero Image */}
          <div className="md:w-7/12 mt-12 md:mt-20 md:absolute md:right-0 md:translate-x-[10%] lg:translate-x-[15%] xl:translate-x-[20%] z-0" >
            <div className="neo-border bg-white p-0 relative hover:shadow-lg transition-all duration-300">
              <img src="/hero-ss.png" alt="NeoBase Chat" className="w-full h-auto"  />
            </div>
          </div>
        </div>
      </div>
    </section>
  );
};

export default HeroSection; 