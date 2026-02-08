import { api } from './api.js';

async function loadFoods() {
    try {
        const foods = await api.getFoods();
        const foodsList = document.getElementById('foods-list');
        foodsList.innerHTML = '';

        if (foods && foods.length > 0) {
            foods.forEach(food => {
                const li = document.createElement('li');
                li.innerHTML = `
                    <div class="food-info">
                        <strong>${food.name}</strong><br>
                        <small>${Math.round(food.calories)} kcal | P: ${Math.round(food.protein)}g | C: ${Math.round(food.carbs)}g | F: ${Math.round(food.fat)}g</small>
                    </div>
                `;
                // Add delete button
                const deleteBtn = document.createElement('button');
                deleteBtn.textContent = 'Delete';
                deleteBtn.onclick = () => deleteFood(food.id);
                li.appendChild(deleteBtn);

                foodsList.appendChild(li);
            });
        } else {
            foodsList.innerHTML = '<li>No foods found.</li>';
        }
    } catch (e) {
        console.error("Failed to load foods:", e);
    }
}

async function createFood(e) {
    e.preventDefault();
    const form = e.target;
    const foodData = {
        name: form.name.value,
        calories: parseFloat(form.calories.value),
        protein: parseFloat(form.protein.value),
        carbs: parseFloat(form.carbs.value),
        fat: parseFloat(form.fat.value),
    };

    try {
        await api.createFood(foodData);
        form.reset();
        loadFoods();
    } catch (e) {
        alert("Failed to create food: " + e.message);
    }
}

async function deleteFood(id) {
    if (!confirm("Are you sure you want to delete this food?")) return;
    try {
        await api.deleteFood(id);
        loadFoods();
    } catch (e) {
        alert("Failed to delete food: " + e.message);
    }
}

window.addEventListener('load', () => {
    loadFoods();
    document.getElementById('create-food-form').addEventListener('submit', createFood);
});
