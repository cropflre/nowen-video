import React from 'react';
import LibraryFormModal from './LibraryFormModal';

interface LibraryModalProps {
    onClose: () => void;
    onSuccess: () => void;
}

const LibraryModal: React.FC<LibraryModalProps> = ({ onClose, onSuccess }) => (
    <LibraryFormModal
        mode="create"
        onClose={onClose}
        onSaved={onSuccess}
    />
);

export default LibraryModal;
