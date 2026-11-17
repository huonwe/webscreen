export function addControlButton(id, title, path, className = '') {
    const existing = document.getElementById(id);
    if (existing) return existing;

    const controls = document.querySelector('.controls-container');
    if (!controls) throw new Error('Controls container is not available');

    const button = document.createElement('button');
    button.id = id;
    button.type = 'button';
    button.className = `control-btn ${className}`.trim();
    button.title = title;
    button.setAttribute('aria-label', title);
    button.innerHTML = `
        <svg viewBox="0 0 24 24" width="24" height="24" fill="currentColor">
            <path d="${path}"></path>
        </svg>`;
    controls.appendChild(button);
    return button;
}

export function addControlSeparator() {
    const controls = document.querySelector('.controls-container');
    if (!controls) throw new Error('Controls container is not available');

    const separator = document.createElement('div');
    separator.className = 'separator';
    controls.appendChild(separator);
    return separator;
}
