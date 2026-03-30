import { api } from './api.js';
import { showToast, showConfirm, renderCalorieGoalBar } from './ui.js';
import { getLocalDateString } from './utils.js';
import { FoodSearch } from './food-search.js';

let currentDate = getLocalDateString();
let selectedFood = null;
let foodSearch = null;
let goalProfile = null;
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
            onSelect: (food) => {
                selectedFood = food;
                const unitLabel = document.getElementById('amount-unit-label');
                if (unitLabel) unitLabel.textContent = food.measurement_unit ? `(${food.measurement_unit})` : '';
            }
        }
    );

    // Fetch calorie goal profile
    goalProfile = await api.getProfile().catch(() => null);

    // Initial load of logs
    loadLogs();

    // Form submit
    document.getElementById('add-log-form').addEventListener('submit', addLog);

    // Mode toggling
    const modeRadios = document.getElementsByName('entry_mode');
    modeRadios.forEach(radio => {
        radio.addEventListener('change', toggleMode);
    });
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
                    const unit = log.food.measurement_unit || 'serving';
                    div.appendChild(document.createTextNode(` — ${Math.round(log.amount * 10) / 10} ${unit}`));
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
        const bar = renderCalorieGoalBar(totalCals, goalProfile?.calorie_goal ?? null);
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

window.addEventListener('load', init);
