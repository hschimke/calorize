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
        dialog.setAttribute('role', 'alertdialog');
        dialog.setAttribute('aria-modal', 'true');
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
        dialog.setAttribute('role', 'alertdialog');
        dialog.setAttribute('aria-modal', 'true');
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

// ============================================================
// Calorie Goal Bar
// ============================================================

/**
 * Renders a calorie progress bar toward a goal.
 * @param {number} consumed - Calories consumed.
 * @param {number|null} goal - Daily calorie goal. Returns null if not set.
 * @param {boolean} [clownMode] - When true and over goal, show clown emoji row instead.
 * @returns {HTMLElement|null}
 */
export function renderCalorieGoalBar(consumed, goal, clownMode = false) {
    if (goal == null) return null;

    const over = consumed > goal;

    if (clownMode && over) {
        const wrap = document.createElement('div');
        wrap.className = 'goal-bar-wrap clown-bar';
        const count = Math.min(10, Math.ceil((consumed / goal) * 5));
        for (let i = 0; i < count; i++) {
            const emoji = document.createElement('span');
            emoji.className = 'clown-emoji';
            emoji.textContent = '\uD83E\uDD21';
            wrap.appendChild(emoji);
        }
        return wrap;
    }

    const pct = Math.min(100, Math.round((consumed / goal) * 100));

    const wrap = document.createElement('div');
    wrap.className = 'goal-bar-wrap';

    const bar = document.createElement('div');
    bar.className = 'goal-bar';

    const fill = document.createElement('div');
    fill.className = 'goal-bar-fill' + (over ? ' over' : '');
    fill.style.width = `${pct}%`;
    bar.appendChild(fill);

    const label = document.createElement('div');
    label.className = 'goal-bar-label' + (over ? ' over' : '');
    label.textContent = over
        ? `${Math.round(consumed - goal)} over goal`
        : `${Math.round(goal - consumed)} remaining`;

    wrap.appendChild(bar);
    wrap.appendChild(label);
    return wrap;
}

// ============================================================
// DOM Buffer
// ============================================================

export class DOMBuffer {
    constructor(targetElement) {
        this.target = targetElement;
        this.fragment = document.createDocumentFragment();
    }

    append(child) {
        this.fragment.appendChild(child);
    }

    flush() {
        this.target.appendChild(this.fragment);
    }

    clearAndFlush() {
        this.target.replaceChildren();
        this.flush();
    }
}
