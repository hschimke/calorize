import { api } from './api.js';
import { showToast, showConfirm, showInput, DOMBuffer } from './ui.js';

async function loadPasskeys() {
    const list = document.getElementById('passkey-list');
    const buffer = new DOMBuffer(list);

    let passkeys;
    try {
        passkeys = await api.getPasskeys();
    } catch (e) {
        const li = document.createElement('li');
        li.textContent = 'Failed to load passkeys.';
        buffer.append(li);
        buffer.clearAndFlush();
        showToast('Failed to load passkeys', 'error');
        return;
    }

    if (!passkeys || passkeys.length === 0) {
        const li = document.createElement('li');
        li.textContent = 'No passkeys found.';
        buffer.append(li);
        buffer.clearAndFlush();
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
        buffer.append(li);
    }
    buffer.clearAndFlush();
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

function updateDerived(daily) {
    const el = document.getElementById('goal-derived');
    if (!daily || daily <= 0) {
        el.textContent = '';
        return;
    }
    const weekly = (daily * 7).toLocaleString();
    const monthly = Math.round(daily * 30.4).toLocaleString();
    el.textContent = `Weekly: ${weekly} kcal · Monthly: ~${monthly} kcal`;
}

const SOURCE_NAMES = {
    afcd: 'AFCD (Australian Food Composition)',
    fdc: 'FDC (USDA Food Data Central)',
    off: 'Open Food Facts',
};

function renderSourceToggles(available, disabled) {
    const container = document.getElementById('source-toggles');
    const buffer = new DOMBuffer(container);

    if (available.length === 0) {
        const msg = document.createElement('p');
        msg.className = 'form-hint';
        msg.textContent = 'No imported food sources found.';
        buffer.append(msg);
        buffer.clearAndFlush();
        return;
    }

    const disabledSet = new Set(disabled);
    for (const source of available) {
        const row = document.createElement('label');
        row.className = 'checkbox-label';

        const checkbox = document.createElement('input');
        checkbox.type = 'checkbox';
        checkbox.dataset.source = source;
        checkbox.checked = !disabledSet.has(source);

        const text = document.createTextNode(SOURCE_NAMES[source] || source);

        row.appendChild(checkbox);
        row.appendChild(text);
        buffer.append(row);
    }
    buffer.clearAndFlush();
}

async function loadPreferences() {
    try {
        const prefs = await api.getPreferences();
        renderSourceToggles(prefs.available_sources || [], prefs.disabled_sources || []);

        document.getElementById('hide-public-user-foods-toggle').checked = !!prefs.hide_public_user_foods;

        const clownSection = document.getElementById('clown-mode-section');
        if (prefs.clown_mode_available) {
            clownSection.hidden = false;
            document.getElementById('clown-mode-toggle').checked = !!prefs.clown_mode;
        }
    } catch (e) {
        showToast('Failed to load preferences', 'error');
    }
}

async function savePreferences() {
    const sourceCheckboxes = document.querySelectorAll('#source-toggles input[type="checkbox"]');
    const disabledSources = [];
    for (const cb of sourceCheckboxes) {
        if (!cb.checked) {
            disabledSources.push(cb.dataset.source);
        }
    }

    const hidePublicUserFoods = document.getElementById('hide-public-user-foods-toggle').checked;
    const clownModeToggle = document.getElementById('clown-mode-toggle');
    const clownMode = clownModeToggle ? clownModeToggle.checked : false;

    try {
        await api.updatePreferences({ clown_mode: clownMode, hide_public_user_foods: hidePublicUserFoods, disabled_sources: disabledSources });
        showToast('Preferences saved');
    } catch (e) {
        showToast(e.message || 'Failed to save preferences', 'error');
    }
}

function updateWeightUnitLabels(unit) {
    const labels = document.querySelectorAll('.weight-unit-label');
    for (const label of labels) {
        label.textContent = unit;
    }
}

async function loadProfile() {
    try {
        const profile = await api.getProfile();
        if (profile) {
            if (profile.calorie_goal != null) {
                document.getElementById('calorie-goal-input').value = profile.calorie_goal;
                updateDerived(profile.calorie_goal);
            }
            if (profile.weight_goal != null) {
                document.getElementById('weight-goal-input').value = profile.weight_goal;
            }
            if (profile.weight_unit) {
                document.getElementById('weight-unit-select').value = profile.weight_unit;
                updateWeightUnitLabels(profile.weight_unit);
            }
        }
    } catch (e) {
        showToast('Failed to load profile', 'error');
    }
}

async function saveGoal() {
    const input = document.getElementById('calorie-goal-input');
    const raw = input.value.trim();
    const goal = raw === '' ? null : parseInt(raw, 10);
    if (raw !== '' && (isNaN(goal) || goal <= 0)) {
        showToast('Please enter a valid calorie goal', 'error');
        return;
    }
    try {
        await api.updateProfile({ calorie_goal: goal });
        showToast('Goal saved');
        updateDerived(goal);
    } catch (e) {
        showToast(e.message || 'Failed to save goal', 'error');
    }
}

async function saveWeightSettings() {
    const goalInput = document.getElementById('weight-goal-input');
    const unitSelect = document.getElementById('weight-unit-select');
    
    const goalRaw = goalInput.value.trim();
    const goal = goalRaw === '' ? null : parseFloat(goalRaw);
    if (goalRaw !== '' && (isNaN(goal) || goal <= 0)) {
        showToast('Please enter a valid weight goal', 'error');
        return;
    }
    
    const unit = unitSelect.value;
    if (unit !== 'kg' && unit !== 'lbs') {
        showToast('Please select a valid unit', 'error');
        return;
    }
    
    try {
        await api.updateProfile({ weight_goal: goal, weight_unit: unit });
        showToast('Weight settings saved');
        updateWeightUnitLabels(unit);
    } catch (e) {
        showToast(e.message || 'Failed to save weight settings', 'error');
    }
}

document.addEventListener('DOMContentLoaded', () => {
    loadPasskeys();
    loadProfile();
    loadPreferences();
    document.getElementById('btn-add-passkey').addEventListener('click', addPasskey);
    document.getElementById('calorie-goal-input').addEventListener('input', (e) => {
        const val = parseInt(e.target.value, 10);
        updateDerived(isNaN(val) ? null : val);
    });
    document.getElementById('btn-save-goal').addEventListener('click', saveGoal);
    document.getElementById('weight-unit-select').addEventListener('change', (e) => {
        updateWeightUnitLabels(e.target.value);
    });
    document.getElementById('btn-save-weight-settings').addEventListener('click', saveWeightSettings);
    document.getElementById('btn-save-preferences').addEventListener('click', savePreferences);
});
