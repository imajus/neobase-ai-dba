import Navbar from './components/Navbar'
import HeroSection from './components/HeroSection'
import VideoSection from './components/VideoSection'
import SupportedTechnologiesSection from './components/SupportedTechnologiesSection'
import Footer from './components/Footer'
import CompactFeaturesSection from './components/CompactFeaturesSection'
import HowItWorksSection from './components/HowItWorksSection'

function App() {

  return (
    <div className="min-h-screen bg-[#FFDB58]/10 overflow-hidden">
      <Navbar />
      <main>
        <HeroSection />
        <VideoSection />
        <SupportedTechnologiesSection />
        <CompactFeaturesSection />
        <HowItWorksSection />
      </main>
      <Footer />
    </div>
  )
}

export default App
