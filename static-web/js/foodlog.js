import { api } from './api.js';

let currentDate = new Date().toISOString().split('T')[0];

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
        select.innerHTML = '<option value="">Select a food...</option>';

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
        logsList.innerHTML = '';

        if (logs && logs.length > 0) {
            logs.forEach(log => {
                const li = document.createElement('li');

                let text = '';
                if (log.food) {
                    text = `<strong>${log.food.name}</strong> - ${Math.round(log.amount * 10) / 10}x`;
                } else {
                    text = 'Quick Add';
                }

                if (log.calories) {
                    text += ` (${Math.round(log.calories)} kcal)`;
                }

                if (log.meal_tag) {
                    text += ` <span style="background:#eee; padding:2px 6px; border-radius:4px; font-size:0.8em; margin-left:10px;">${log.meal_tag}</span>`;
                }

                const div = document.createElement('div');
                div.innerHTML = text;

                const deleteBtn = document.createElement('button');
                deleteBtn.textContent = 'Remove';
                deleteBtn.style.marginLeft = '10px';
                deleteBtn.onclick = () => deleteLog(log.id);

                li.appendChild(div);
                li.appendChild(deleteBtn);
                logsList.appendChild(li);
            });
        } else {
            logsList.innerHTML = '<li>No logs for this date.</li>';
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

    const logData = {
        amount: parseFloat(form.amount.value),
        meal_tag: form.meal_tag.value,
        // The backend expects 'logged_at' for the date/time. 
        // We Append T12:00:00Z to ensure it's treated as a specific date, or just let backend handle it?
        // Let's use the date picker value. 
        logged_at: new Date(currentDate).toISOString()
    };

    if (mode === 'food') {
        logData.food_id = form.food_id.value;
    } else {
        const cals = parseFloat(form.calories.value);
        const amt = parseFloat(form.amount.value);
        logData.calories = cals * amt;
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
        alert("Failed to create log: " + e.message);
    }
}

async function deleteLog(id) {
    if (!confirm("Remove this log?")) return;
    try {
        await api.deleteLog(id);
        loadLogs();
    } catch (e) {
        alert("Failed to remove log: " + e.message);
    }
}

window.addEventListener('load', init);
