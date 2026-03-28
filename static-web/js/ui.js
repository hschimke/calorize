// ============================================================
// Toast Notifications
// ============================================================

function getOrCreateToastContainer() {
    let container = document.querySelector('.toast-container');
    if (!container) {
        container = document.createElement('div');
        container.className = 'toast-container';
        document.body.appendChild(container);
    }
    return container;
}

export function showToast(message, type = 'success') {
    const container = getOrCreateToastContainer();
    const toast = document.createElement('div');
    toast.className = `toast toast-${type}`;
    toast.textContent = message;
    container.appendChild(toast);
    setTimeout(() => {
        toast.remove();
    }, 3000);
}

// ============================================================
// Confirm Dialog
// ============================================================

export function showConfirm(message) {
    return new Promise((resolve) => {
        const overlay = document.createElement('div');
        overlay.style.cssText = [
            'position: fixed',
            'inset: 0',
            'background: rgba(0,0,0,0.4)',
            'display: flex',
            'align-items: center',
            'justify-content: center',
            'z-index: 2000',
        ].join('; ');

        const dialog = document.createElement('div');
        dialog.style.cssText = [
            'background: var(--color-surface)',
            'border-radius: var(--radius-md)',
            'padding: var(--space-6)',
            'max-width: 360px',
            'width: 90%',
            'box-shadow: 0 8px 32px rgba(0,0,0,0.2)',
        ].join('; ');

        const msg = document.createElement('p');
        msg.textContent = message;
        msg.style.cssText = 'margin: 0 0 var(--space-5); font-size: 1rem; color: var(--color-text);';

        const actions = document.createElement('div');
        actions.style.cssText = 'display: flex; gap: var(--space-3); justify-content: flex-end;';

        const cancelBtn = document.createElement('button');
        cancelBtn.textContent = 'Cancel';
        cancelBtn.className = 'btn btn-secondary';
        cancelBtn.onclick = () => { overlay.remove(); resolve(false); };

        const confirmBtn = document.createElement('button');
        confirmBtn.textContent = 'Confirm';
        confirmBtn.className = 'btn btn-danger';
        confirmBtn.onclick = () => { overlay.remove(); resolve(true); };

        actions.appendChild(cancelBtn);
        actions.appendChild(confirmBtn);
        dialog.appendChild(msg);
        dialog.appendChild(actions);
        overlay.appendChild(dialog);
        document.body.appendChild(overlay);

        overlay.addEventListener('click', (e) => {
            if (e.target === overlay) { overlay.remove(); resolve(false); }
        });
    });
}

// ============================================================
// Input Dialog
// ============================================================

export function showInput(message, defaultValue = '') {
    return new Promise((resolve) => {
        const overlay = document.createElement('div');
        overlay.style.cssText = [
            'position: fixed',
            'inset: 0',
            'background: rgba(0,0,0,0.4)',
            'display: flex',
            'align-items: center',
            'justify-content: center',
            'z-index: 2000',
        ].join('; ');

        const dialog = document.createElement('div');
        dialog.style.cssText = [
            'background: var(--color-surface)',
            'border-radius: var(--radius-md)',
            'padding: var(--space-6)',
            'max-width: 360px',
            'width: 90%',
            'box-shadow: 0 8px 32px rgba(0,0,0,0.2)',
        ].join('; ');

        const msg = document.createElement('p');
        msg.textContent = message;
        msg.style.cssText = 'margin: 0 0 var(--space-4); font-size: 1rem; color: var(--color-text);';

        const input = document.createElement('input');
        input.type = 'text';
        input.value = defaultValue;
        input.style.cssText = 'display: block; width: 100%; box-sizing: border-box; margin-bottom: var(--space-5);';

        const actions = document.createElement('div');
        actions.style.cssText = 'display: flex; gap: var(--space-3); justify-content: flex-end;';

        const cancelBtn = document.createElement('button');
        cancelBtn.textContent = 'Cancel';
        cancelBtn.className = 'btn btn-secondary';
        cancelBtn.onclick = () => { overlay.remove(); resolve(null); };

        const saveBtn = document.createElement('button');
        saveBtn.textContent = 'Save';
        saveBtn.className = 'btn btn-primary';
        saveBtn.onclick = () => { overlay.remove(); resolve(input.value.trim() || null); };

        input.addEventListener('keydown', (e) => {
            if (e.key === 'Enter') saveBtn.click();
            if (e.key === 'Escape') cancelBtn.click();
        });

        actions.appendChild(cancelBtn);
        actions.appendChild(saveBtn);
        dialog.appendChild(msg);
        dialog.appendChild(input);
        dialog.appendChild(actions);
        overlay.appendChild(dialog);
        document.body.appendChild(overlay);

        input.focus();
        input.select();

        overlay.addEventListener('click', (e) => {
            if (e.target === overlay) { overlay.remove(); resolve(null); }
        });
    });
}
