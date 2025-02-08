import React, { useState } from 'react';
import { AlertTriangle, X } from 'lucide-react';

interface DeleteConnectionModalProps {
  connectionName: string;
  onConfirm: () => void;
  onCancel: () => void;
}

export default function DeleteConnectionModal({
  connectionName,
  onConfirm,
  onCancel,
}: DeleteConnectionModalProps) {
  const [confirmText, setConfirmText] = useState('');
  const isConfirmValid = confirmText === 'delete';

  return (
    <div className="fixed inset-0 bg-black/50 backdrop-blur-sm flex items-center justify-center p-4 z-50">
      <div className="bg-white neo-border rounded-lg w-full max-w-md">
        <div className="flex justify-between items-center p-6 border-b-4 border-black">
          <div className="flex items-center gap-3">
            <AlertTriangle className="w-6 h-6 text-neo-error" />
            <h2 className="text-2xl font-bold">Delete Connection</h2>
          </div>
          <button
            onClick={onCancel}
            className="hover:bg-neo-gray rounded-lg p-2 transition-colors"
          >
            <X className="w-6 h-6" />
          </button>
        </div>

        <div className="p-6">
          <p className="text-gray-600 mb-6">
            Are you sure you want to delete the connection to <strong>{connectionName}</strong>? 
            This action cannot be undone.
          </p>
          
          <div className="mb-6">
            <label className="block text-sm font-medium text-gray-700 mb-2">
              Type <span className="font-mono bg-gray-100 px-2 py-1 rounded">delete</span> to confirm:
            </label>
            <input
              type="text"
              value={confirmText}
              onChange={(e) => setConfirmText(e.target.value)}
              className="neo-input w-full"
              placeholder="Type 'delete' to confirm"
              autoFocus
            />
          </div>

          <div className="flex gap-4">
            <button
              onClick={onConfirm}
              disabled={!isConfirmValid}
              className={`neo-border px-4 py-2 font-bold text-base transition-all flex-1 ${
                isConfirmValid
                  ? 'bg-neo-error text-white hover:translate-y-[-2px] hover:shadow-[6px_6px_0px_0px_rgba(0,0,0,1)] active:translate-y-[0px] active:shadow-[4px_4px_0px_0px_rgba(0,0,0,1)]'
                  : 'bg-gray-200 text-gray-400 cursor-not-allowed'
              }`}
            >
              Delete Connection
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