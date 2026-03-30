import { api } from './api.js';
import { getLocalDateString } from './utils.js';
import { renderCalorieGoalBar } from './ui.js';

async function updateDashboard() {
    try {
        const today = getLocalDateString();
        const [stats, profile, logs] = await Promise.all([
            api.getStats('day', today),
            api.getProfile(),
            api.getLogs(today),
        ]);

        const calories = Math.round(stats?.calories || 0);
        document.getElementById('dashboard-calories').textContent = calories;
        document.getElementById('dashboard-protein').textContent = Math.round(stats?.protein || 0);
        document.getElementById('dashboard-carbs').textContent = Math.round(stats?.carbs || 0);
        document.getElementById('dashboard-fat').textContent = Math.round(stats?.fat || 0);

        const calCard = document.getElementById('dashboard-calories').parentElement;
        calCard.querySelector('.goal-bar-wrap')?.remove();
        const bar = renderCalorieGoalBar(calories, profile?.calorie_goal ?? null);
        if (bar) calCard.appendChild(bar);

        const logsList = document.getElementById('dashboard-logs-list');
        logsList.innerHTML = '';

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
                logsList.appendChild(li);
            });
        } else {
            logsList.innerHTML = '<li>No logs for today</li>';
        }

    } catch (error) {
        console.error("Failed to load dashboard data:", error);
    }
}

window.addEventListener('load', updateDashboard);