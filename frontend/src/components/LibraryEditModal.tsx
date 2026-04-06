import React from 'react';
import LibraryFormModal from './LibraryFormModal';

interface LibraryEditModalProps {
    library: any;
    onClose: () => void;
    onSaved: () => void;
    onDeleted: () => void;
}

const LibraryEditModal: React.FC<LibraryEditModalProps> = ({ library, onClose, onSaved, onDeleted }) => (
    <LibraryFormModal
        mode="edit"
        library={library}
        onClose={onClose}
        onSaved={onSaved}
        onDeleted={onDeleted}
    />
);

export default LibraryEditModal;
