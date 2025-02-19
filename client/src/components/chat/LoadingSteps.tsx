import { Check, Loader2 } from 'lucide-react';

interface Step {
    text: string;
    done: boolean;
}

interface LoadingStepsProps {
    steps: Step[];
}

export default function LoadingSteps({ steps }: LoadingStepsProps) {
    return (
        <div className="flex flex-col space-y-4">
            {steps.map((step, index) => (
                <div key={index} className="flex items-start gap-2">
                    <div className="mt-1 flex-shrink-0">
                        {step.done ? (
                            <div className="w-4 h-4 rounded-full bg-green-500 flex items-center justify-center">
                                <Check className="w-3 h-3 text-white stroke-[3]" />
                            </div>
                        ) : (
                            <Loader2 className="w-4 h-4 text-gray-700 animate-spin" />
                        )}
                    </div>
                    <p className={`
                        text-gray-700 
                        whitespace-pre-wrap
                        ${step.done ? 'text-gray-500' : 'text-black'}
                    `}>
                        {step.text}
                    </p>
                </div>
            ))}
        </div>
    );
} 