import { api } from './api.js';
import { getLocalDateString } from './utils.js';
import { renderCalorieGoalBar, showToast, DOMBuffer } from './ui.js';

let weightUnit = 'kg';

async function updateDashboard() {
    try {
        const today = getLocalDateString();
        const [profile, userPrefs, stats, logs, weightStats] = await Promise.all([
            api.getProfile().catch(() => null),
            api.getPreferences().catch(() => null),
            api.getStats('day', today),
            api.getLogs(today),
            api.getWeightStats().catch(() => null),
        ]);

        const calories = Math.round(stats?.calories || 0);
        document.getElementById('dashboard-calories').textContent = calories;
        document.getElementById('dashboard-protein').textContent = Math.round(stats?.protein || 0);
        document.getElementById('dashboard-carbs').textContent = Math.round(stats?.carbs || 0);
        document.getElementById('dashboard-fat').textContent = Math.round(stats?.fat || 0);

        const calCard = document.getElementById('dashboard-calories').parentElement;
        calCard.querySelector('.goal-bar-wrap')?.remove();
        const bar = renderCalorieGoalBar(calories, profile?.calorie_goal ?? null, userPrefs?.clown_mode ?? false);
        if (bar) calCard.appendChild(bar);

        // Update Weight Card Info
        weightUnit = weightStats?.weight_unit || profile?.weight_unit || 'kg';
        const labels = document.querySelectorAll('.weight-unit-label');
        for (const label of labels) {
            label.textContent = weightUnit;
        }

        const weightSpan = document.getElementById('dashboard-weight');
        if (weightStats && weightStats.current_weight > 0) {
            weightSpan.textContent = weightStats.current_weight.toFixed(1);
        } else {
            weightSpan.textContent = '—';
        }

        const logsList = document.getElementById('dashboard-logs-list');
        const buffer = new DOMBuffer(logsList);

        if (logs && logs.length > 0) {
            logs.forEach(log => {
                const li = document.createElement('li');
                let text = '';
                if (log.food) {
                    text = `${log.food.name} (${log.amount}x)`;
                } else {
                    text = 'Quick Add';
                }
                if (log.calories) {
                    text += ` - ${Math.round(log.calories)} kcal`;
                }
                if (log.meal_tag) {
                    text += ` [${log.meal_tag}]`;
                }
                li.textContent = text;
                buffer.append(li);
            });
        } else {
            const li = document.createElement('li');
            li.textContent = 'No logs for today';
            buffer.append(li);
        }
        buffer.clearAndFlush();

    } catch (error) {
        console.error("Failed to load dashboard data:", error);
        showToast("Failed to load dashboard data", "error");
    }
}

function init() {
    const weightForm = document.getElementById('dashboard-weight-form');
    if (weightForm) {
        weightForm.addEventListener('submit', async (e) => {
            e.preventDefault();
            const input = document.getElementById('dashboard-weight-input');
            const weight = parseFloat(input.value);
            if (isNaN(weight) || weight <= 0) {
                showToast("Please enter a valid weight", "error");
                return;
            }
            try {
                await api.createWeightLog({
                    weight,
                    unit: weightUnit,
                    logged_at: new Date().toISOString()
                });
                showToast("Weight logged successfully");
                input.value = '';
                await updateDashboard();
            } catch (err) {
                showToast(err.message || "Failed to log weight", "error");
            }
        });
    }
    updateDashboard();
}

window.addEventListener('load', init);