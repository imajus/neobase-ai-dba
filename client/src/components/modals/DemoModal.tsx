import React, { useEffect } from 'react';
import { X } from 'lucide-react';

interface DemoModalProps {
  isOpen: boolean;
  onClose: () => void;
}

declare global {
  interface Window {
    Supademo: any;
  }
}

export function DemoModal({ isOpen, onClose }: DemoModalProps) {
  useEffect(() => {
    const initSupademo = () => {
      if (typeof window.Supademo === 'function') {
        window.Supademo("8f6c44979ed62e95c859d7cb3822bceb7546722e3160297baf14b358e832499d", {
          variables: {
            email: "",
            name: ""
          }
        });
      } else {
        // Retry after a short delay if Supademo is not loaded yet
        setTimeout(initSupademo, 100);
      }
    };

    initSupademo();
  }, []);

  useEffect(() => {
    if (isOpen && typeof window.Supademo === 'function') {
      window.Supademo("8f6c44979ed62e95c859d7cb3822bceb7546722e3160297baf14b358e832499d")
        .loadDemo("cm7u1fvg42cw2hilggnjt2kox");
    }
  }, [isOpen]);

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50 backdrop-blur-sm animate-fade-in">
      <div className="relative w-full max-w-7xl bg-white dark:bg-surface-dark rounded-2xl shadow-2xl animate-scale-in">
        <button
          onClick={onClose}
          className="absolute -top-4 -right-4 w-10 h-10 bg-white dark:bg-surface-dark rounded-full shadow-lg flex items-center justify-center hover:scale-110 transition-transform duration-300 z-[60]"
        >
          <X className="w-6 h-6 text-text-primary-light dark:text-text-primary-dark" />
        </button>

        <div style={{ 
          position: 'relative', 
          boxSizing: 'content-box', 
          maxHeight: '80vh', 
          width: '100%', 
          aspectRatio: '1.9021134593993325', 
          padding: '40px 0 40px 0' 
        }}>
          <iframe 
            src="https://app.supademo.com/embed/cm7u1fvg42cw2hilggnjt2kox?embed_v=2" 
            loading="lazy" 
            title="Neobase Demo" 
            allow="clipboard-write" 
            frameBorder="0" 
            style={{ position: 'absolute', top: 0, left: 0, width: '100%', height: '100%' }}
            className="rounded-xl"
          />
        </div>
      </div>
    </div>
  );
} 