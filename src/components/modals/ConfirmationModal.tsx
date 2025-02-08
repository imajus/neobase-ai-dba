import { AlertTriangle, X } from 'lucide-react';

interface ConfirmationModalProps {
  title: string;
  message: string;
  onConfirm: () => void;
  onCancel: () => void;
}

export default function ConfirmationModal({
  title,
  message,
  onConfirm,
  onCancel,
}: ConfirmationModalProps) {
  return (
    <div className="fixed inset-0 bg-black/50 backdrop-blur-sm flex items-center justify-center p-4 z-50">
      <div className="bg-white neo-border rounded-lg w-full max-w-md">
        <div className="flex justify-between items-center p-6 border-b-4 border-black">
          <div className="flex items-center gap-3">
            <AlertTriangle className="w-6 h-6 text-neo-error" />
            <h2 className="text-2xl font-bold">{title}</h2>
          </div>
          <button
            onClick={onCancel}
            className="hover:bg-neo-gray rounded-lg p-2 transition-colors"
          >
            <X className="w-6 h-6" />
          </button>
        </div>

        <div className="p-6">
          <p className="text-gray-600 mb-6">{message}</p>

          <div className="flex gap-4">
            <button
              onClick={onConfirm}
              className="neo-border bg-neo-error text-white px-4 py-2 font-bold text-base transition-all hover:translate-y-[-2px] hover:shadow-[6px_6px_0px_0px_rgba(0,0,0,1)] active:translate-y-[0px] active:shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] flex-1"
            >
              Confirm
            </button>
            <button
              onClick={onCancel}
              className="neo-button-secondary flex-1"
            >
              Cancel
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}