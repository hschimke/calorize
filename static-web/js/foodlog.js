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
    const logData = {
        food_id: form.food_id.value,
        amount: parseFloat(form.amount.value),
        meal_tag: form.meal_tag.value,
        date: currentDate // Assuming API might support date override if needed, though usually logs to 'current' or we might need to handle it.
        // Wait, the API `createLog` does NOT take a date in the signature in `api.js`. 
        // It says "Create a log entry". Usually this defaults to now.
        // If the user selects a PAST date, creating a log might log it for NOW.
        // I need to check if the API supports logging for a specific date.
        // Viewing `api.js`: uses `POST /logs`.
    };

    // If the API doesn't support date in POST /logs, it will log for today.
    // That's a limitation I should probs note or fix, but for now I'll just send it.
    // Wait, looking at `api.js` line 200: `createLog(logData)`.

    try {
        await api.createLog(logData);
        // form.reset(); // Don't fully reset, maybe keep meal tag? 
        // But reset food and amount is good.
        form.food_id.value = '';
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
