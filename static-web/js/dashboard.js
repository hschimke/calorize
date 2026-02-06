import { api } from './api.js';

async function updateDashboard() {
    try {
        // 1. Fetch Stats
        // We assume 'day' period returns today's stats by default if no date provided, 
        // or we can explicitly pass today's date. The API seems to handle defaulting.
        const stats = await api.getStats('day');

        if (stats) {
            document.getElementById('dashboard-calories').textContent = Math.round(stats.calories || 0);
            document.getElementById('dashboard-protein').textContent = Math.round(stats.protein || 0);
            document.getElementById('dashboard-carbs').textContent = Math.round(stats.carbs || 0);
            document.getElementById('dashboard-fat').textContent = Math.round(stats.fat || 0);
        }

        // 2. Fetch Logs
        const logs = await api.getLogs();
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