import { api } from './api.js';
import { showToast, showConfirm, showInput } from './ui.js';

async function loadPasskeys() {
    const list = document.getElementById('passkey-list');
    list.textContent = '';

    let passkeys;
    try {
        passkeys = await api.getPasskeys();
    } catch (e) {
        list.innerHTML = '<li>Failed to load passkeys.</li>';
        showToast('Failed to load passkeys', 'error');
        return;
    }

    if (!passkeys || passkeys.length === 0) {
        list.innerHTML = '<li>No passkeys found.</li>';
        return;
    }

    for (const pk of passkeys) {
        const li = document.createElement('li');

        const info = document.createElement('div');
        info.className = 'food-info';

        const nameEl = document.createElement('strong');
        nameEl.id = `pk-name-${pk.id}`;
        nameEl.textContent = pk.name;
        info.appendChild(nameEl);

        const detailEl = document.createElement('small');
        detailEl.className = 'color-text-muted';
        const createdDate = new Date(pk.created_at).toLocaleDateString();
        const lastUsedDate = new Date(pk.last_used_at).toLocaleDateString();
        detailEl.textContent = ` · Added ${createdDate} · Last used ${lastUsedDate}`;
        info.appendChild(detailEl);

        const actions = document.createElement('div');
        actions.className = 'food-actions';

        const renameBtn = document.createElement('button');
        renameBtn.className = 'btn btn-secondary btn-sm';
        renameBtn.textContent = 'Rename';
        renameBtn.addEventListener('click', () => renamePasskey(pk.id, pk.name));

        const deleteBtn = document.createElement('button');
        deleteBtn.className = 'btn btn-danger btn-sm';
        deleteBtn.textContent = 'Delete';
        deleteBtn.addEventListener('click', () => deletePasskey(pk.id));

        actions.appendChild(renameBtn);
        actions.appendChild(deleteBtn);

        li.appendChild(info);
        li.appendChild(actions);
        list.appendChild(li);
    }
}

async function deletePasskey(id) {
    const ok = await showConfirm('Delete this passkey? You will not be able to log in with it anymore.');
    if (!ok) return;
    try {
        await api.deletePasskey(id);
        showToast('Passkey deleted');
        loadPasskeys();
    } catch (e) {
        showToast(e.message || 'Failed to delete passkey', 'error');
    }
}

async function renamePasskey(id, currentName) {
    const name = await showInput('New name for this passkey:', currentName);
    if (!name) return;
    try {
        await api.renamePasskey(id, name);
        showToast('Passkey renamed');
        loadPasskeys();
    } catch (e) {
        showToast(e.message || 'Failed to rename passkey', 'error');
    }
}

async function addPasskey() {
    try {
        await api.addPasskey();
        showToast('Passkey added');
        loadPasskeys();
    } catch (e) {
        showToast(e.message || 'Failed to add passkey', 'error');
    }
}

document.addEventListener('DOMContentLoaded', () => {
    loadPasskeys();
    document.getElementById('btn-add-passkey').addEventListener('click', addPasskey);
});
