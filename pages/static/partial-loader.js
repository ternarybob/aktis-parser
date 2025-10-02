// Partial loader for including HTML fragments
async function loadPartial(selector, partialPath) {
    try {
        const response = await fetch(partialPath);
        if (!response.ok) {
            console.error(`Failed to load partial: ${partialPath}`);
            return;
        }
        const html = await response.text();
        const element = document.querySelector(selector);
        if (element) {
            element.innerHTML = html;

            // Execute any scripts in the partial
            const scripts = element.querySelectorAll('script');
            scripts.forEach(script => {
                const newScript = document.createElement('script');
                if (script.src) {
                    newScript.src = script.src;
                } else {
                    newScript.textContent = script.textContent;
                }
                document.body.appendChild(newScript);
                script.remove();
            });
        }
    } catch (error) {
        console.error(`Error loading partial ${partialPath}:`, error);
    }
}
