import { Github, Menu, X } from 'lucide-react'
import { useEffect, useState } from 'react'

const Navbar = () => {
  const [isMenuOpen, setIsMenuOpen] = useState(false)
  const [forkCount, setForkCount] = useState<number | null>(null)

  useEffect(() => {
    const fetchForkCount = async () => {
      try {
          const response = await fetch('https://api.github.com/repos/bhaskarblur/neobase-ai-dba');
          const data = await response.json();
          setForkCount(data.forks_count);
      } catch (error) {
          console.error('Error fetching fork count:', error);
          setForkCount(1); // Default value if API call fails
      }
  };

    fetchForkCount();
  }, [])

  const formatForkCount = (count: number): string => {
    if (count >= 1000) {
      return `${(count / 1000).toFixed(1)}k`
    }
    return count.toString()
  }

  const toggleMenu = () => {
    setIsMenuOpen(!isMenuOpen)
  }

  return (
    <>
      <nav className="py-4 px-6 md:px-8 lg:px-12 border-b-4 border-black bg-white fixed top-0 left-0 right-0 z-[100] shadow-md">
        <div className="container mx-auto max-w-7xl">
          <div className="flex items-center justify-between">
            {/* Logo */}
            <a href="/" className="flex items-center gap-2">
              <img src="/neobase-logo.svg" alt="NeoBase Logo" className="w-10 h-10" />
              <span className="text-2xl font-bold">NeoBase</span>
            </a>

            {/* Desktop Navigation */}
            <div className="hidden md:flex items-center gap-6">
              <a href="#features" className="font-medium hover:text-gray-600 transition-colors">Features</a>
              <a href="#technologies" className="font-medium hover:text-gray-600 transition-colors">Technologies</a>
              
              {/* Product Hunt Button */}
              <a 
                href="https://www.producthunt.com/posts/neobase" 
                target="_blank" 
                rel="noopener noreferrer"
                className="neo-button flex items-center gap-2 py-2 px-4 text-sm bg-[#DA552F] text-white"
              >
                <svg className="w-4 h-4" viewBox="0 0 512 512" fill="currentColor">
                  <path d="M256 0C114.615 0 0 114.615 0 256s114.615 256 256 256 256-114.615 256-256S397.385 0 256 0zm-96 320h-32v-32h32v32zm0-80h-32v-32h32v32zm0-80h-32v-32h32v32zm64 160h-32v-32h32v32zm0-80h-32v-32h32v32zm0-80h-32v-32h32v32zm32 160V128h32v96h32v96h-64zm96 0h-32v-32h32v32zm0-80h-32v-32h32v32zm0-80h-32v-32h32v32zm0-80h-32V80h32v32zm64 240h-32v-32h32v32zm0-80h-32v-32h32v32zm0-80h-32v-32h32v32z"/>
                </svg>
                <span>Product Hunt</span>
              </a>
              
              {/* Github Fork Button */}
              <a 
                href="https://github.com/bhaskarblur/neobase-ai-dba" 
                target="_blank" 
                rel="noopener noreferrer"
                className="neo-button flex items-center gap-2 py-2 px-4 text-sm bg-black text-white"
              >
                <Github className="w-4 h-4" />
                <span>Fork Us</span>
                <span className="bg-white/20 px-2 py-0.5 rounded-full text-xs font-mono">
                  {formatForkCount(forkCount || 1)}
                </span>
              </a>
              
            </div>

            {/* Mobile Menu Button */}
            <button 
              className="md:hidden p-2 neo-border bg-white"
              onClick={toggleMenu}
              aria-label="Toggle menu"
            >
              {isMenuOpen ? <X className="w-5 h-5" /> : <Menu className="w-5 h-5" />}
            </button>
          </div>

          {/* Mobile Navigation */}
          {isMenuOpen && (
            <div className="md:hidden mt-4 py-4 border-t border-gray-200">
              <div className="flex flex-col gap-4">
                <a href="#features" className="font-medium hover:text-gray-600 transition-colors py-2">Features</a>
                <a href="#technologies" className="font-medium hover:text-gray-600 transition-colors py-2">Technologies</a>
                
                <div className="flex flex-col gap-3 mt-2">
                  {/* Product Hunt Button */}
                  <a 
                    href="https://www.producthunt.com/posts/neobase" 
                    target="_blank" 
                    rel="noopener noreferrer"
                    className="neo-button flex items-center justify-center gap-2 py-2 bg-[#DA552F] text-white"
                  >
                    <svg className="w-4 h-4" viewBox="0 0 512 512" fill="currentColor">
                      <path d="M256 0C114.615 0 0 114.615 0 256s114.615 256 256 256 256-114.615 256-256S397.385 0 256 0zm-96 320h-32v-32h32v32zm0-80h-32v-32h32v32zm0-80h-32v-32h32v32zm64 160h-32v-32h32v32zm0-80h-32v-32h32v32zm0-80h-32v-32h32v32zm32 160V128h32v96h32v96h-64zm96 0h-32v-32h32v32zm0-80h-32v-32h32v32zm0-80h-32v-32h32v32zm0-80h-32V80h32v32zm64 240h-32v-32h32v32zm0-80h-32v-32h32v32zm0-80h-32v-32h32v32z"/>
                    </svg>
                    <span>Product Hunt</span>
                  </a>
                  
                  {/* Github Fork Button */}
                  <a 
                    href="https://github.com/bhaskarblur/neobase-ai-dba" 
                    target="_blank" 
                    rel="noopener noreferrer"
                    className="neo-button flex items-center justify-center gap-2 py-2 bg-black text-white"
                  >
                    <Github className="w-4 h-4" />
                    <span>Fork Us</span>
                    <span className="bg-white/20 px-2 py-0.5 rounded-full text-xs font-mono">
                      {formatForkCount(forkCount || 1)}
                    </span>
                  </a>
                  
                  {/* Get Started Button */}
                  <a 
                    href="https://github.com/bhaskarblur/neobase-ai-dba" 
                    target="_blank" 
                    rel="noopener noreferrer"
                    className="neo-button py-2 flex items-center justify-center"
                  >
                    Get Started
                  </a>
                </div>
              </div>
            </div>
          )}
        </div>
      </nav>
      {/* Spacer to prevent content from being hidden under the fixed navbar */}
      <div className="h-[73px]"></div>
    </>
  )
}

export default Navbar 