import { CheckCircle2, X } from 'lucide-react';
import { useEffect, useState } from 'react';

interface SuccessBannerProps {
    message: string;
    onClose: () => void;
    duration?: number;
}

export default function SuccessBanner({ message, onClose, duration = 2000 }: SuccessBannerProps) {
    const [isVisible, setIsVisible] = useState(false);

    useEffect(() => {
        // Trigger slide-in animation
        setTimeout(() => setIsVisible(true), 100);

        // Auto close after duration
        const timer = setTimeout(() => {
            setIsVisible(false);
            setTimeout(onClose, 300); // Wait for slide-out animation
        }, duration);

        return () => clearTimeout(timer);
    }, [duration, onClose]);

    return (
        <div
            className={`
        fixed top-0 left-1/2 -translate-x-1/2 
        transition-all duration-300 ease-in-out
        ${isVisible ? 'translate-y-0' : '-translate-y-full'}
      `}
        >
            <div className="flex items-center gap-2 px-6 py-3 bg-black text-white rounded-b-lg shadow-lg">
                <CheckCircle2 className="w-5 h-5 text-[#FFDB58]" />
                <span className="font-medium">{message}</span>
                <button
                    onClick={() => {
                        setIsVisible(false);
                        setTimeout(onClose, 300);
                    }}
                    className="ml-4 hover:opacity-80"
                >
                    <X className="w-4 h-4" />
                </button>
            </div>
        </div>
    );
} 