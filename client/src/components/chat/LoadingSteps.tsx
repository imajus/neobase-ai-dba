import { Check, Loader } from 'lucide-react';

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
                            <Check className="w-5 h-5 text-green-500 stroke-[3]" />
                        ) : (
                            <Loader className="w-5 h-5 animate-spin" />
                        )}
                    </div>
                    <p className={`
                        text-black
                        whitespace-pre-wrap
                        ${step.done ? 'font-normal' : 'font-semibold'}
                    `}>
                        {step.text}
                    </p>
                </div>
            ))
            }
        </div >
    );
} 