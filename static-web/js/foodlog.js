import { api } from './api.js';
import { showToast, showConfirm } from './ui.js';
import { getLocalDateString } from './utils.js';

let currentDate = getLocalDateString();

async function init() {
    // Setup date picker
    const datePicker = document.getElementById('log-date');
    datePicker.value = currentDate;
    datePicker.addEventListener('change', (e) => {
        currentDate = e.target.value;
        loadLogs();
    });

    // Populate foods
    await loadFoods();

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
    const foodSelect = document.getElementById('food-select');
    const caloriesInput = document.getElementById('calories-input');

    if (mode === 'food') {
        groupFood.style.display = 'block';
        groupCalories.style.display = 'none';
        foodSelect.required = true;
        caloriesInput.required = false;
        caloriesInput.value = '';
    } else {
        groupFood.style.display = 'none';
        groupCalories.style.display = 'block';
        foodSelect.required = false;
        caloriesInput.required = true;
        foodSelect.value = '';
    }
}

async function loadFoods() {
    try {
        const foods = await api.getFoods();
        const select = document.getElementById('food-select');
        select.textContent = '';
        const defaultOption = document.createElement('option');
        defaultOption.value = '';
        defaultOption.textContent = 'Select a food...';
        select.appendChild(defaultOption);

        if (foods) {
            foods.forEach(food => {
                const option = document.createElement('option');
                option.value = food.id;
                option.textContent = `${food.name} (${Math.round(food.calories)} kcal)`;
                select.appendChild(option);
            });
        }
    } catch (e) {
        console.error("Failed to load foods:", e);
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
                    div.appendChild(document.createTextNode(` - ${Math.round(log.amount * 10) / 10}x`));
                } else {
                    nameSpan.textContent = 'Quick Add';
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
    if (currentDate === getLocalDateString(now)) {
        logged_at = now.toISOString();
    } else {
        // Approximate noon local time for a past/future selected date
        logged_at = new Date(`${currentDate}T12:00:00`).toISOString();
    }

    const logData = {
        amount: parseFloat(form.amount.value),
        meal_tag: form.meal_tag.value,
        logged_at: logged_at
    };

    if (mode === 'food') {
        logData.food_id = form.food_id.value;
    } else {
        const cals = parseFloat(form.calories.value);
        const amt = parseFloat(form.amount.value);
        logData.calories = cals * amt;
        logData.amount = 1; // Calories already pre-multiplied
    }

    try {
        await api.createLog(logData);

        // Reset specific fields
        if (mode === 'food') {
            form.food_id.value = '';
        } else {
            form.calories.value = '';
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
