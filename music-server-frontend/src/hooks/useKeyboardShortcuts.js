import { useEffect } from 'react';

/**
 * Hook to add keyboard shortcuts for media player controls
 * Inspired by LMS (Lightweight Music Server)
 * 
 * Shortcuts:
 * - Space: Play/Pause
 * - Ctrl + Left: Previous track
 * - Ctrl + Right: Next track
 * - Ctrl + Up: Increase volume (if available)
 * - Ctrl + Down: Decrease volume (if available)
 */
export function useKeyboardShortcuts({ 
    onPlayPause, 
    onPrevious, 
    onNext, 
    onVolumeUp, 
    onVolumeDown 
}) {
    useEffect(() => {
        const handleKeyDown = (event) => {
            // Ignore if user is typing in an input field
            if (event.target instanceof HTMLInputElement || 
                event.target.tagName === 'TEXTAREA' ||
                event.target.isContentEditable) {
                return;
            }

            let handled = false;

            // Space: Play/Pause
            if (event.code === 'Space' && !event.ctrlKey && !event.shiftKey && !event.altKey) {
                event.preventDefault();
                onPlayPause?.();
                handled = true;
            }
            // Ctrl + Left Arrow: Previous track
            else if (event.code === 'ArrowLeft' && event.ctrlKey && !event.shiftKey) {
                event.preventDefault();
                onPrevious?.();
                handled = true;
            }
            // Ctrl + Right Arrow: Next track
            else if (event.code === 'ArrowRight' && event.ctrlKey && !event.shiftKey) {
                event.preventDefault();
                onNext?.();
                handled = true;
            }
            // Ctrl + Up Arrow: Volume up
            else if (event.code === 'ArrowUp' && event.ctrlKey && !event.shiftKey) {
                event.preventDefault();
                onVolumeUp?.();
                handled = true;
            }
            // Ctrl + Down Arrow: Volume down
            else if (event.code === 'ArrowDown' && event.ctrlKey && !event.shiftKey) {
                event.preventDefault();
                onVolumeDown?.();
                handled = true;
            }

            if (handled) {
                // Show a brief toast notification (optional)
                console.log('Keyboard shortcut executed');
            }
        };

        document.addEventListener('keydown', handleKeyDown);

        return () => {
            document.removeEventListener('keydown', handleKeyDown);
        };
    }, [onPlayPause, onPrevious, onNext, onVolumeUp, onVolumeDown]);
}
