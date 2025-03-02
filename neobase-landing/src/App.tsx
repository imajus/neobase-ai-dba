import Navbar from './components/Navbar'
import HeroSection from './components/HeroSection'
import VideoSection from './components/VideoSection'
import SupportedTechnologiesSection from './components/SupportedTechnologiesSection'
import Footer from './components/Footer'
import CompactFeaturesSection from './components/CompactFeaturesSection'
import HowItWorksSection from './components/HowItWorksSection'
import Clarity from '@microsoft/clarity';
import { initializeApp } from "firebase/app";
import { getAnalytics, logEvent } from "firebase/analytics";
import { useEffect } from 'react';

function App() {
  useEffect(() => {
    initializeAnalytics();
  }, []);

  return (
    <div className="min-h-screen bg-[#FFDB58]/10 overflow-hidden">
      <Navbar />
      <main className="flex flex-col space-y-8 md:space-y-0">
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

function initializeAnalytics() {
  // Initialize Microsoft Clarity
  if (import.meta.env.CLARITY_PROJECT_ID) {
    Clarity.init(import.meta.env.CLARITY_PROJECT_ID);
    console.log('Clarity initialized');
  }

  // Initialize Firebase Analytics
  const firebaseConfig = {
    apiKey: import.meta.env.FIREBASE_API_KEY,
    authDomain: import.meta.env.FIREBASE_AUTH_DOMAIN,
    projectId: import.meta.env.FIREBASE_PROJECT_ID,
    storageBucket: import.meta.env.FIREBASE_STORAGE_BUCKET,
    messagingSenderId: import.meta.env.FIREBASE_MESSAGING_SENDER_ID,
    appId: import.meta.env.FIREBASE_APP_ID,
    measurementId: import.meta.env.FIREBASE_MEASUREMENT_ID
  };

  // Only initialize Firebase if the required environment variables are set
  if (import.meta.env.FIREBASE_API_KEY && import.meta.env.FIREBASE_MEASUREMENT_ID) {
    // Initialize Firebase
    const app = initializeApp(firebaseConfig);
    const analytics = getAnalytics(app);
    
    // Log page view event
    logEvent(analytics, 'page_view');
    
    // Track custom events for different sections
    trackSectionViews(analytics);
  }
}

// Function to track when users view different sections
function trackSectionViews(analytics: any) {
  // Use Intersection Observer to track when sections come into view
  const sections = [
    { id: 'hero', name: 'hero_section_view' },
    { id: 'video', name: 'video_section_view' },
    { id: 'technologies', name: 'technologies_section_view' },
    { id: 'features', name: 'features_section_view' },
    { id: 'how-it-works', name: 'how_it_works_section_view' }
  ];

  sections.forEach(section => {
    const element = document.getElementById(section.id);
    if (element) {
      const observer = new IntersectionObserver(
        (entries) => {
          entries.forEach(entry => {
            if (entry.isIntersecting) {
              logEvent(analytics, section.name);
              observer.unobserve(entry.target); // Only track once
            }
          });
        },
        { threshold: 0.5 } // Fire when 50% of the element is visible
      );
      observer.observe(element);
    }
  });
}

export default App
