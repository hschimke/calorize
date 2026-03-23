import { api } from './api.js';
import { getLocalDateString } from './utils.js';

async function updateDashboard() {
    try {
        // 1. Fetch Stats
        const today = getLocalDateString();
        const stats = await api.getStats('day', today);

        if (stats) {
            document.getElementById('dashboard-calories').textContent = Math.round(stats.calories || 0);
            document.getElementById('dashboard-protein').textContent = Math.round(stats.protein || 0);
            document.getElementById('dashboard-carbs').textContent = Math.round(stats.carbs || 0);
            document.getElementById('dashboard-fat').textContent = Math.round(stats.fat || 0);
        }

        // 2. Fetch Logs
        const logs = await api.getLogs(today);
        const logsList = document.getElementById('dashboard-logs-list');
        logsList.innerHTML = ''; // Clear loading state

        if (logs && logs.length > 0) {
            logs.forEach(log => {
                const li = document.createElement('li');
                // Construct display string
                let text = '';
                if (log.food) {
                    text = `${log.food.name} (${log.amount}x)`;
                } else {
                    text = 'Quick Add';
                }

                // Add calories info
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