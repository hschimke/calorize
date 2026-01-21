import { request } from './api.js';
import { injectNavbar } from './components.js';

// Inject Navbar
injectNavbar();

const state = {
    date: new Date().toISOString().split('T')[0],
    userID: localStorage.getItem('user_id')
};

// Redirect if not logged in (basic check)
if (!state.userID) {
    window.location.href = '/login.html';
}

const els = {
    dateInput: document.getElementById('dateInput'),
    totalCals: document.getElementById('totalCals'),
    totalProtein: document.getElementById('totalProtein'),
    totalCarbs: document.getElementById('totalCarbs'),
    totalFat: document.getElementById('totalFat'),
    logsTableBody: document.querySelector('#logsTable tbody'),
};

// Set initial date
els.dateInput.value = state.date;

// Event Listeners
els.dateInput.addEventListener('change', (e) => {
    state.date = e.target.value;
    loadData();
});

async function loadData() {
    await Promise.all([loadStats(), loadLogs()]);
}

async function loadStats() {
    try {
        const stats = await request(`/stats?date=${state.date}&user_id=${state.userID}`);
        if (stats) {
            els.totalCals.textContent = Math.round(stats.total_calories);
            els.totalProtein.textContent = Math.round(stats.total_protein) + 'g';
            els.totalCarbs.textContent = Math.round(stats.total_carbs) + 'g';
            els.totalFat.textContent = Math.round(stats.total_fat) + 'g';
        }
    } catch (err) {
        console.error('Failed to load stats', err);
    }
}

async function loadLogs() {
    try {
        const logs = await request(`/logs?date=${state.date}&user_id=${state.userID}`);
        els.logsTableBody.innerHTML = '';

        if (!logs || logs.length === 0) {
            els.logsTableBody.innerHTML = '<tr><td colspan="4" class="text-center">No logs for this day</td></tr>';
            return;
        }

        logs.forEach(log => {
            const row = document.createElement('tr');
            // Assuming log has: Food.Name, Amount, MealTag, Calories
            // We might need to adjust based on actual API response structure (checking joins)
            // If the API only returns FoodID, we can't show name. 
            // I'll assume standard API returns joined data or I need to fetch it.
            // For now, fail gracefully if name missing.
            const foodName = log.food_name || log.Food?.Name || log.food_id;
            const cals = Math.round(log.calories || 0);

            row.innerHTML = `
                <td>${log.meal_tag || '-'}</td>
                <td>${foodName}</td>
                <td>${log.amount}</td>
                <td>${cals}</td>
                <td>
                    <button class="danger small delete-btn" data-id="${log.id}">×</button>
                </td>
            `;
            els.logsTableBody.appendChild(row);
        });

        // Add delete handlers
        document.querySelectorAll('.delete-btn').forEach(btn => {
            btn.addEventListener('click', async (e) => {
                if (confirm('Delete this log?')) {
                    await deleteLog(e.target.dataset.id);
                }
            });
        });

    } catch (err) {
        console.error('Failed to load logs', err);
    }
}

async function deleteLog(id) {
    try {
        await request(`/logs/${id}`, { method: 'DELETE' });
        loadData();
    } catch (err) {
        alert('Failed to delete log');
    }
}

// Initial Load
loadData();
