import { Pause, Play } from 'lucide-react';
import { useState, useRef } from 'react';
import FloatingBackground from './FloatingBackground';

const VideoSection = () => {
  const [isPlaying, setIsPlaying] = useState(false);
  const videoRef = useRef<HTMLVideoElement>(null);

  const togglePlayPause = () => {
    if (videoRef.current) {
      if (isPlaying) {
        videoRef.current.pause();
      } else {
        videoRef.current.play();
      }
      setIsPlaying(!isPlaying);
    }
  };

  return (
    <section className="py-32 md:py-36 lg:py-36 relative overflow-hidden bg-[#FFDB58]/5">
      <FloatingBackground count={15} opacity={0.03} />
      <div className="container mx-auto px-6 md:px-8 relative">
        <h2 className="text-4xl md:text-5xl lg:text-5xl font-bold text-center mb-20">
          See NeoBase in <span className="text-yellow-500">Action</span>
        </h2>
        
        <div className="relative">
          {/* Video Container */}
          <div className="max-w-5xl mx-auto relative z-10">
            <div className="neo-border bg-white p-4 hover:shadow-xl transition-all duration-300" style={{ transform: 'rotate(-1deg)' }}>
              {/* Video */}
              <div className="relative overflow-hidden neo-border" style={{ minHeight: '560px' }}>
                <video 
                  ref={videoRef}
                  className="w-full h-auto"
                  poster="/video-poster.png"
                >
                  <source src="/demo.mp4" type="video/mp4" />
                  Your browser does not support the video tag.
                </video>
                
                {/* Play/Pause Button */}
                <button 
                  onClick={togglePlayPause}
                  className="absolute top-1/2 left-1/2 transform -translate-x-1/2 -translate-y-1/2 bg-[#FFDB58] hover:bg-[#FFDB58]/90 text-black neo-border rounded-full p-6 transition-all"
                  aria-label={isPlaying ? "Pause video" : "Play video"}
                >
                  {isPlaying ? (
                    <Pause className="w-10 h-10" />
                  ) : (
                    <Play className="w-10 h-10 ml-1" />
                  )}
                </button>
              </div>
            </div>
          </div>
          
          {/* Feature Callouts with Arrows */}
          
          {/* Right Feature */}
          <div className="absolute right-0 top-1/2 transform translate-x-[20%] -translate-y-1/2 z-0 hidden md:block">
            <div className="flex items-center">
              <svg width="120" height="100" viewBox="0 0 120 100" fill="none" xmlns="http://www.w3.org/2000/svg">
                <path d="M120 50 L0 50" stroke="black" strokeWidth="6" fill="none" strokeLinecap="round"/>
                <polygon points="0,50 25,35 25,65" fill="black"/>
              </svg>
              <div className="neo-border bg-[#FFDB58] px-6 py-3 font-bold text-xl text-center ml-6 whitespace-nowrap">
                Visualize & Export Results
              </div>
            </div>
          </div>
          
          {/* Bottom Feature */}
          <div className="absolute bottom-0 left-1/2 transform -translate-x-1/2 translate-y-[60%] z-0 hidden md:block">
            <div className="flex flex-col items-center">
              <svg width="100" height="120" viewBox="0 0 100 120" fill="none" xmlns="http://www.w3.org/2000/svg">
                <path d="M50 120 L50 0" stroke="black" strokeWidth="6" fill="none" strokeLinecap="round"/>
                <polygon points="50,0 35,25 65,25" fill="black"/>
              </svg>
              <div className="neo-border bg-[#FFDB58] px-6 py-3 font-bold text-xl text-center mt-6 whitespace-nowrap">
              Talk to Your Database Naturally
              </div>
            </div>
          </div>
          
          {/* Left Feature */}
          <div className="absolute left-0 top-1/2 transform -translate-x-[20%] -translate-y-1/2 z-0 hidden md:block">
            <div className="flex items-center">
              <div className="neo-border bg-[#FFDB58] px-6 py-3 font-bold text-xl text-center mr-6 whitespace-nowrap">
                AI-Driven Operations
              </div>
              <svg width="120" height="100" viewBox="0 0 120 100" fill="none" xmlns="http://www.w3.org/2000/svg">
                <path d="M0 50 L120 50" stroke="black" strokeWidth="6" fill="none" strokeLinecap="round"/>
                <polygon points="120,50 95,35 95,65" fill="black"/>
              </svg>
            </div>
          </div>
          
          {/* Mobile Features (simplified) */}
          <div className="md:hidden space-y-6 mt-12">
            <div className="neo-border bg-[#FFDB58] px-6 py-3 font-bold text-lg text-center">
              AI-Driven Operations
            </div>
            <div className="neo-border bg-[#FFDB58] px-6 py-3 font-bold text-lg text-center">
              Visualize & Export Results
            </div>
            <div className="neo-border bg-[#FFDB58] px-6 py-3 font-bold text-lg text-center">
            Talk to Your Database Naturally
            </div>
          </div>
        </div>
      </div>
    </section>
  );
};

export default VideoSection; 