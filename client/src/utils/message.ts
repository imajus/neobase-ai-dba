
// Formats to 15 Mar 2025, 12:00
export const formatActionAt = (actionAt: string) => {
	const date = new Date(actionAt);
	if (isNaN(date.getTime())) {
		return "";
	}
	// Format to 15 Mar 2025, 12:00
	return date.toLocaleString('en-US', {
		month: 'short',
		day: 'numeric',
		year: 'numeric',
		hour: '2-digit',
		minute: '2-digit',
	});
};

