import Config from './config.js';

const Logger = {
    log: (level, message, ...args) => {
        if (!Config.DEBUG && level !== 'error') return;

        const timestamp = new Date().toISOString();
        const prefix = `[${timestamp}] [${level.toUpperCase()}]`;

        switch (level) {
            case 'info':
                console.info(prefix, message, ...args);
                break;
            case 'warn':
                console.warn(prefix, message, ...args);
                break;
            case 'error':
                console.error(prefix, message, ...args);
                break;
            case 'debug':
                console.debug(prefix, message, ...args);
                break;
            case 'trace':
                console.trace(prefix, message, ...args);
                break;
            default:
                console.log(prefix, message, ...args);
        }
    },

    info: (message, ...args) => Logger.log('info', message, ...args),
    warn: (message, ...args) => Logger.log('warn', message, ...args),
    error: (message, ...args) => Logger.log('error', message, ...args),
    debug: (message, ...args) => Logger.log('debug', message, ...args),
    trace: (message, ...args) => Logger.log('trace', message, ...args),
};

export default Logger;
