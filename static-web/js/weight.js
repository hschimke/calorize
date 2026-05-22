import { api } from './api.js';
import { showToast, showConfirm, DOMBuffer } from './ui.js';
import { getLocalDateString } from './utils.js';
import { drawWeightChart } from './charts.js';

let userProfile = null;
let editingLogId = null;

async function init() {
    // Set default date to today
    const dateInput = document.getElementById('log-date-input');
    dateInput.value = getLocalDateString();

    try {
        // Fetch profile to get default weight unit and goal
        userProfile = await api.getProfile();
        if (userProfile) {
            const unit = userProfile.weight_unit || 'kg';
            document.getElementById('log-unit-select').value = unit;
            updateWeightUnitLabels(unit);
        }
    } catch (e) {
        console.warn("Failed to load user profile", e);
    }

    // Set up form submission
    const form = document.getElementById('log-weight-form');
    form.addEventListener('submit', handleFormSubmit);

    // Set up cancel edit button
    const cancelBtn = document.getElementById('btn-cancel-edit');
    cancelBtn.addEventListener('click', cancelEdit);

    // Load logs and statistics
    await loadData();
}

function updateWeightUnitLabels(unit) {
    const labels = document.querySelectorAll('.weight-unit-label');
    for (const label of labels) {
        label.textContent = unit;
    }
}

async function loadData() {
    try {
        const [logs, stats] = await Promise.all([
            api.getWeightLogs(0),
            api.getWeightStats().catch(() => null)
        ]);

        const preferredUnit = (userProfile && userProfile.weight_unit) ? userProfile.weight_unit : 'kg';
        const goalWeight = (userProfile && userProfile.weight_goal) ? userProfile.weight_goal : null;

        renderStats(stats, logs, preferredUnit, goalWeight);
        renderLogs(logs);

        // Render weight history chart
        const canvas = document.getElementById('chart-weight');
        if (canvas) {
            drawWeightChart(canvas, logs, goalWeight, preferredUnit);
        }
    } catch (e) {
        console.error("Failed to load weight data", e);
        showToast("Failed to load weight data", "error");
    }
}

function renderStats(stats, logs, preferredUnit, goalWeight) {
    const currentSpan = document.getElementById('weight-current');
    const goalSpan = document.getElementById('weight-goal');
    const changeSpan = document.getElementById('weight-change');
    const progressSpan = document.getElementById('weight-progress');

    // Default preferred unit labels on UI
    updateWeightUnitLabels(preferredUnit);

    if (stats && logs.length > 0) {
        currentSpan.textContent = stats.current_weight.toFixed(1);
        
        if (stats.goal_weight != null) {
            goalSpan.textContent = stats.goal_weight.toFixed(1);
        } else {
            goalSpan.textContent = '—';
        }

        const change = stats.weight_change;
        const sign = change > 0 ? '+' : '';
        changeSpan.textContent = `${sign}${change.toFixed(1)}`;

        if (stats.goal_weight != null) {
            progressSpan.textContent = `${Math.round(stats.goal_progress)}%`;
        } else {
            progressSpan.textContent = '—';
        }
    } else {
        currentSpan.textContent = '—';
        goalSpan.textContent = goalWeight != null ? goalWeight.toFixed(1) : '—';
        changeSpan.textContent = '—';
        progressSpan.textContent = '—';
    }
}

function renderLogs(logs) {
    const list = document.getElementById('weight-logs-list');
    const buffer = new DOMBuffer(list);

    if (!logs || logs.length === 0) {
        const li = document.createElement('li');
        li.textContent = 'No weight logs found.';
        buffer.append(li);
        buffer.clearAndFlush();
        return;
    }

    logs.forEach(log => {
        const li = document.createElement('li');

        const infoDiv = document.createElement('div');
        const dateText = new Date(log.logged_at).toLocaleDateString(undefined, {
            year: 'numeric',
            month: 'short',
            day: 'numeric'
        });

        const strong = document.createElement('strong');
        strong.textContent = `${log.weight} ${log.unit}`;
        
        const dateSmall = document.createElement('small');
        dateSmall.textContent = ` · ${dateText}`;

        infoDiv.appendChild(strong);
        infoDiv.appendChild(dateSmall);

        const actionDiv = document.createElement('div');
        actionDiv.className = 'food-actions';

        const editBtn = document.createElement('button');
        editBtn.className = 'btn btn-secondary btn-sm';
        editBtn.textContent = 'Edit';
        editBtn.addEventListener('click', () => startEdit(log));

        const deleteBtn = document.createElement('button');
        deleteBtn.className = 'btn btn-danger btn-sm';
        deleteBtn.textContent = 'Remove';
        deleteBtn.addEventListener('click', () => deleteWeightLog(log.id));

        actionDiv.appendChild(editBtn);
        actionDiv.appendChild(deleteBtn);

        li.appendChild(infoDiv);
        li.appendChild(actionDiv);
        buffer.append(li);
    });

    buffer.clearAndFlush();
}

async function handleFormSubmit(e) {
    e.preventDefault();

    const weightInput = document.getElementById('log-weight-input');
    const unitSelect = document.getElementById('log-unit-select');
    const dateInput = document.getElementById('log-date-input');

    const weight = parseFloat(weightInput.value);
    const unit = unitSelect.value;
    const dateVal = dateInput.value;

    if (isNaN(weight) || weight <= 0) {
        showToast("Please enter a valid weight value.", "error");
        return;
    }

    let logged_at;
    const now = new Date();
    const todayStr = getLocalDateString(now);

    if (dateVal === todayStr) {
        // For today, use the current precise time
        logged_at = now.toISOString();
    } else {
        // For other dates, use local noon to avoid day-boundary shifts when converted to UTC
        const [year, month, day] = dateVal.split('-').map(Number);
        const localNoon = new Date(year, month - 1, day, 12, 0, 0);
        logged_at = localNoon.toISOString();
    }

    try {
        if (editingLogId) {
            await api.updateWeightLog(editingLogId, { weight, unit, logged_at });
            showToast("Weight log updated");
        } else {
            await api.createWeightLog({ weight, unit, logged_at });
            showToast("Weight log created");
        }
        cancelEdit();
        await loadData();
    } catch (err) {
        showToast(err.message || "Failed to save weight log", "error");
    }
}

function startEdit(log) {
    editingLogId = log.id;

    document.getElementById('log-id-input').value = log.id;
    document.getElementById('log-weight-input').value = log.weight;
    document.getElementById('log-unit-select').value = log.unit;
    
    // Format ISO logged_at to YYYY-MM-DD for picker
    const dateStr = log.logged_at.slice(0, 10);
    document.getElementById('log-date-input').value = dateStr;

    document.getElementById('log-form-title').textContent = "Edit Weight Log";
    document.getElementById('btn-submit-weight').textContent = "Update Weight";
    
    const cancelBtn = document.getElementById('btn-cancel-edit');
    cancelBtn.removeAttribute('hidden');

    // Scroll form into view
    document.getElementById('log-weight-form').scrollIntoView({ behavior: 'smooth' });
}

function cancelEdit() {
    editingLogId = null;

    document.getElementById('log-id-input').value = '';
    document.getElementById('log-weight-input').value = '';
    document.getElementById('log-date-input').value = getLocalDateString();
    
    if (userProfile && userProfile.weight_unit) {
        document.getElementById('log-unit-select').value = userProfile.weight_unit;
    } else {
        document.getElementById('log-unit-select').value = 'kg';
    }

    document.getElementById('log-form-title').textContent = "Log Weight";
    document.getElementById('btn-submit-weight').textContent = "Log Weight";
    
    const cancelBtn = document.getElementById('btn-cancel-edit');
    cancelBtn.setAttribute('hidden', '');
}

async function deleteWeightLog(id) {
    const confirmed = await showConfirm("Remove this weight log?");
    if (!confirmed) return;
    try {
        await api.deleteWeightLog(id);
        showToast("Weight log removed");
        if (editingLogId === id) {
            cancelEdit();
        }
        await loadData();
    } catch (e) {
        showToast(e.message || "Failed to remove weight log", "error");
    }
}

document.addEventListener('DOMContentLoaded', init);
