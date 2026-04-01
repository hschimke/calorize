import { api } from './api.js';
import { showToast, showConfirm, renderCalorieGoalBar } from './ui.js';
import { getLocalDateString } from './utils.js';
import { FoodSearch } from './food-search.js';

let currentDate = getLocalDateString();
let selectedFood = null;
let previewGeneration = 0;
let foodSearch = null;
let goalProfile = null;
let userPrefs = null;
const noteInput = document.getElementById('note-input');

async function init() {
    // Setup date picker
    const datePicker = document.getElementById('log-date');
    datePicker.value = currentDate;
    datePicker.addEventListener('change', (e) => {
        currentDate = e.target.value;
        loadLogs();
    });

    // Create food search component
    foodSearch = new FoodSearch(
        document.getElementById('food-search-container'),
        {
            onSelect: async (food) => {
                selectedFood = food;
                const unitLabel = document.getElementById('amount-unit-label');
                const portionGroup = document.getElementById('group-portion');
                const portionSelect = document.getElementById('portion-select');

                while (portionSelect.firstChild) portionSelect.removeChild(portionSelect.firstChild);
                portionGroup.style.display = 'none';

                if (unitLabel) unitLabel.textContent = food.measurement_unit ? `(${food.measurement_unit})` : '';

                // Search results don't include portions — fetch full food detail
                const fullFood = await api.getFood(food.id);
                selectedFood = fullFood;

                if (fullFood.portions && fullFood.portions.length > 0) {
                    portionGroup.style.display = 'block';

                    // Default option (usually 100g base)
                    const defOption = document.createElement('option');
                    defOption.value = '';
                    defOption.textContent = `${fullFood.measurement_amount} ${fullFood.measurement_unit} (base)`;
                    portionSelect.appendChild(defOption);

                    fullFood.portions.forEach(p => {
                        const opt = document.createElement('option');
                        opt.value = p.name;
                        opt.textContent = p.name;
                        portionSelect.appendChild(opt);
                    });
                }
            }
        }
    );

    // Fetch calorie goal profile and preferences
    [goalProfile, userPrefs] = await Promise.all([
        api.getProfile().catch(() => null),
        api.getPreferences().catch(() => null),
    ]);

    // Initial load of logs
    loadLogs();

    // Form submit
    document.getElementById('add-log-form').addEventListener('submit', addLog);

    // Mode toggling
    const modeRadios = document.getElementsByName('entry_mode');
    modeRadios.forEach(radio => {
        radio.addEventListener('change', toggleMode);
    });

    initCopyDialog();
}

function toggleMode(e) {
    const mode = e.target.value;
    const groupFood = document.getElementById('group-food');
    const groupCalories = document.getElementById('group-calories');
    const caloriesInput = document.getElementById('calories-input');

    const unitLabel = document.getElementById('amount-unit-label');
    if (mode === 'food') {
        groupFood.style.display = 'block';
        groupCalories.style.display = 'none';
        caloriesInput.required = false;
        caloriesInput.value = '';
        noteInput.value = '';
    } else {
        groupFood.style.display = 'none';
        groupCalories.style.display = 'block';
        caloriesInput.required = true;
        selectedFood = null;
        if (foodSearch) foodSearch.clear();
        if (unitLabel) unitLabel.textContent = '';
    }
}

async function loadLogs() {
    try {
        const logs = await api.getLogs(currentDate);
        const logsList = document.getElementById('logs-list');
        logsList.textContent = '';

        if (logs && logs.length > 0) {
            logs.forEach(log => {
                const li = document.createElement('li');

                const div = document.createElement('div');

                const nameSpan = document.createElement('strong');
                if (log.food) {
                    nameSpan.textContent = log.food.name;
                    div.appendChild(nameSpan);
                    let amountLabel;
                    if (log.portion_name) {
                        amountLabel = log.amount === 1
                            ? log.portion_name
                            : `${Math.round(log.amount * 10) / 10} × ${log.portion_name}`;
                    } else {
                        const unit = log.food.measurement_unit || 'serving';
                        amountLabel = `${Math.round(log.amount * 10) / 10} ${unit}`;
                    }
                    div.appendChild(document.createTextNode(` — ${amountLabel}`));
                } else {
                    nameSpan.textContent = log.note ? `[qc] ${log.note}` : 'Quick Add';
                    div.appendChild(nameSpan);
                }

                if (log.calories) {
                    div.appendChild(document.createTextNode(` (${Math.round(log.calories)} kcal)`));
                }

                if (log.meal_tag) {
                    const badge = document.createElement('span');
                    badge.className = 'meal-badge';
                    badge.textContent = log.meal_tag;
                    div.appendChild(badge);
                }

                const deleteBtn = document.createElement('button');
                deleteBtn.textContent = 'Remove';
                deleteBtn.className = 'btn btn-danger btn-sm';
                deleteBtn.onclick = () => deleteLog(log.id);

                li.appendChild(div);
                li.appendChild(deleteBtn);
                logsList.appendChild(li);
            });
        } else {
            const li = document.createElement('li');
            li.textContent = 'No logs for this date.';
            logsList.appendChild(li);
        }

        const totalCals = Math.round(
            (logs || []).reduce((sum, log) => sum + (log.calories || 0), 0)
        );
        document.getElementById('foodlog-calories').textContent = totalCals;

        const calCard = document.getElementById('foodlog-cal-card');
        calCard.querySelector('.goal-bar-wrap')?.remove();
        const bar = renderCalorieGoalBar(totalCals, goalProfile?.calorie_goal ?? null, userPrefs?.clown_mode ?? false);
        if (bar) calCard.appendChild(bar);
    } catch (e) {
        console.error("Failed to load logs:", e);
    }
}

async function addLog(e) {
    e.preventDefault();
    const form = e.target;

    // Determine mode
    const mode = document.querySelector('input[name="entry_mode"]:checked').value;

    let logged_at;
    const now = new Date();
    const todayStr = getLocalDateString(now);

    if (currentDate === todayStr) {
        // For today, use the current precise time
        logged_at = now.toISOString();
    } else {
        // For other dates, use local noon to avoid day-boundary shifts when converted to UTC
        const [year, month, day] = currentDate.split('-').map(Number);
        const localNoon = new Date(year, month - 1, day, 12, 0, 0);
        logged_at = localNoon.toISOString();
    }

    const logData = {
        amount: parseFloat(form.amount.value),
        meal_tag: form.meal_tag.value,
        logged_at: logged_at
    };

    if (mode === 'food') {
        if (!selectedFood) {
            showToast("Please select a food.", 'error');
            return;
        }
        logData.food_id = selectedFood.id;
        const portionSelect = document.getElementById('portion-select');
        if (portionSelect && portionSelect.value) {
            logData.portion_name = portionSelect.value;
        }
    } else {
        const cals = parseFloat(form.calories.value);
        const amt = parseFloat(form.amount.value);
        logData.calories = cals * amt;
        logData.amount = 1; // Calories already pre-multiplied
        logData.note = noteInput.value.trim() || null;
    }

    try {
        await api.createLog(logData);

        // Reset specific fields
        if (mode === 'food') {
            selectedFood = null;
            foodSearch.clear();
        } else {
            form.calories.value = '';
            noteInput.value = '';
        }
        form.amount.value = '1';

        loadLogs();
    } catch (e) {
        showToast("Failed to create log: " + e.message, 'error');
    }
}

async function deleteLog(id) {
    const confirmed = await showConfirm("Remove this log?");
    if (!confirmed) return;
    try {
        await api.deleteLog(id);
        loadLogs();
    } catch (e) {
        showToast("Failed to remove log: " + e.message, 'error');
    }
}

function initCopyDialog() {
    const dialog = document.getElementById('copy-day-dialog');
    const openBtn = document.getElementById('copy-day-btn');
    const cancelBtn = document.getElementById('copy-cancel-btn');
    const confirmBtn = document.getElementById('copy-confirm-btn');
    const fromDateInput = document.getElementById('copy-from-date');

    openBtn.addEventListener('click', () => {
        // Default from-date: one day before currentDate
        const d = new Date(currentDate + 'T12:00:00');
        d.setDate(d.getDate() - 1);
        fromDateInput.value = getLocalDateString(d);

        // Reset all meal checkboxes to checked
        document.querySelectorAll('input[name="copy_meal"]').forEach(cb => { cb.checked = true; });

        dialog.showModal();
        loadCopyPreview();
    });

    cancelBtn.addEventListener('click', () => dialog.close());

    // Close on backdrop click
    dialog.addEventListener('click', (e) => {
        if (e.target === dialog) dialog.close();
    });

    fromDateInput.addEventListener('change', loadCopyPreview);
    document.querySelectorAll('input[name="copy_meal"]').forEach(cb => {
        cb.addEventListener('change', loadCopyPreview);
    });

    confirmBtn.addEventListener('click', async () => {
        const fromDate = fromDateInput.value;
        const checkedTags = [...document.querySelectorAll('input[name="copy_meal"]:checked')]
            .map(cb => cb.value);
        confirmBtn.disabled = true;
        try {
            const result = await api.copyLogs(fromDate, currentDate, checkedTags);
            dialog.close();
            loadLogs();
            showToast(`Copied ${result.count} ${result.count === 1 ? 'entry' : 'entries'}`);
        } catch (e) {
            showToast('Failed to copy entries: ' + e.message, 'error');
            confirmBtn.disabled = false;
        }
    });
}

async function loadCopyPreview() {
    const generation = ++previewGeneration;
    const fromDate = document.getElementById('copy-from-date').value;
    const checkedTags = new Set(
        [...document.querySelectorAll('input[name="copy_meal"]:checked')].map(cb => cb.value)
    );
    const preview = document.getElementById('copy-preview');
    const confirmBtn = document.getElementById('copy-confirm-btn');

    if (!fromDate) {
        preview.textContent = '';
        confirmBtn.disabled = true;
        confirmBtn.textContent = 'Copy 0 entries';
        return;
    }

    preview.textContent = 'Loading...';
    confirmBtn.disabled = true;

    let logs;
    try {
        logs = await api.getLogs(fromDate);
    } catch (e) {
        if (generation !== previewGeneration) return;
        preview.textContent = 'Failed to load preview.';
        return;
    }

    if (generation !== previewGeneration) return;

    const filtered = (logs || []).filter(log => checkedTags.has(log.meal_tag));

    if (filtered.length === 0) {
        preview.textContent = 'No entries for selected meals on this date.';
        confirmBtn.textContent = 'Copy 0 entries';
        return;
    }

    // Group by meal tag in display order
    const groups = {};
    for (const log of filtered) {
        (groups[log.meal_tag] = groups[log.meal_tag] || []).push(log);
    }

    preview.textContent = '';
    for (const meal of ['breakfast', 'lunch', 'dinner', 'snack']) {
        if (!groups[meal]) continue;
        const heading = document.createElement('strong');
        heading.textContent = meal.charAt(0).toUpperCase() + meal.slice(1);
        preview.appendChild(heading);
        const ul = document.createElement('ul');
        ul.className = 'copy-preview-list';
        for (const log of groups[meal]) {
            const li = document.createElement('li');
            const name = log.food ? log.food.name : (log.note ? `[qc] ${log.note}` : 'Quick Add');
            const cals = log.calories != null ? ` (${Math.round(log.calories)} kcal)` : '';
            li.textContent = `${name}${cals}`;
            ul.appendChild(li);
        }
        preview.appendChild(ul);
    }

    confirmBtn.disabled = false;
    confirmBtn.textContent = `Copy ${filtered.length} ${filtered.length === 1 ? 'entry' : 'entries'}`;
}

window.addEventListener('load', init);
